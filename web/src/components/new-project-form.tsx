"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Check, Loader2, TriangleAlert } from "lucide-react";
import type { Provider } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { authHeaders } from "@/lib/client-auth";
import { DeployIntoField } from "@/components/deploy-into-field";

const API = "/bff";
const inputCls =
  "h-9 w-full rounded-lg border border-input bg-card px-3 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";

function csv(s: string): string[] {
  return s
    .split(",")
    .map((x) => x.trim())
    .filter(Boolean);
}

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-xs font-medium text-muted-foreground">{label}</span>
      {children}
      {hint && <span className="text-[11px] text-muted-foreground">{hint}</span>}
    </label>
  );
}

export function NewProjectForm({ providers, initialProvider }: { providers: Provider[]; initialProvider?: string }) {
  const router = useRouter();
  const preset = initialProvider && providers.some((p) => p.name === initialProvider) ? initialProvider : undefined;

  // Access-vending works on AWS (IAM Identity Center), Azure (Entra group + Azure
  // RBAC) and GCP (Entra group via Workforce Identity Federation) - all implement
  // the ProjectProvisioner capability.
  const accessProviders = providers.filter((p) => p.type === "aws" || p.type === "azure" || p.type === "gcp");

  const [name, setName] = useState("");
  const [environment, setEnvironment] = useState("dev");
  const [provider, setProvider] = useState(preset ?? accessProviders[0]?.name ?? providers[0]?.name ?? "");
  const [userNames, setUserNames] = useState("");
  // AWS
  const [accountId, setAccountId] = useState("");
  const [permissionSetName, setPermissionSetName] = useState("team-readonly");
  const [managedPolicyArns, setManagedPolicyArns] = useState("arn:aws:iam::aws:policy/ReadOnlyAccess");
  // Azure
  const [subscriptionId, setSubscriptionId] = useState("");
  const [resourceGroup, setResourceGroup] = useState("");
  const [roleName, setRoleName] = useState("Reader");
  const [pimEligible, setPimEligible] = useState(false);
  // GCP (Workforce Identity Federation: Entra group -> GCP project)
  const [gcpRole, setGcpRole] = useState("roles/viewer");
  const [workforcePoolId, setWorkforcePoolId] = useState("");
  const [entraGroupIds, setEntraGroupIds] = useState("");

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);

  const selectedType = providers.find((p) => p.name === provider)?.type;
  const isAzure = selectedType === "azure";
  const isGcp = selectedType === "gcp";

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setOk(null);
    setSubmitting(true);
    try {
      const spec: Record<string, unknown> = isAzure
        ? {
            user_names: csv(userNames),
            role_name: roleName,
            pim_eligible: pimEligible,
            ...(subscriptionId.trim() ? { subscription_id: subscriptionId.trim() } : {}),
            ...(resourceGroup.trim() ? { resource_group: resourceGroup.trim() } : {}),
          }
        : isGcp
        ? {
            role_name: gcpRole,
            entra_group_ids: csv(entraGroupIds),
            ...(accountId.trim() ? { account_id: accountId.trim() } : {}),
            ...(workforcePoolId.trim() ? { workforce_pool_id: workforcePoolId.trim() } : {}),
            ...(userNames.trim() ? { user_names: csv(userNames) } : {}),
          }
        : {
            account_id: accountId,
            user_names: csv(userNames),
            permission_set_name: permissionSetName,
            managed_policy_arns: csv(managedPolicyArns),
          };
      const res = await fetch(`${API}/api/v1/projects`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ name, environment, provider, spec }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `request failed (${res.status})`);
        return;
      }
      setOk(`Project "${data.name}" created - status ${data.status}.`);
      router.refresh();
      setTimeout(() => router.push("/projects"), 900);
    } catch (err) {
      setError(String(err));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form onSubmit={submit} className="space-y-6">
      {error && (
        <div className="flex items-start gap-2 rounded-xl border border-danger/30 bg-danger/10 p-3 text-sm text-danger">
          <TriangleAlert className="mt-0.5 size-4 shrink-0" />
          <span>{error}</span>
        </div>
      )}
      {ok && (
        <div className="flex items-start gap-2 rounded-xl border border-success/30 bg-success/10 p-3 text-sm text-success">
          <Check className="mt-0.5 size-4 shrink-0" />
          <span>{ok}</span>
        </div>
      )}

      <Card>
        <CardHeader>
          <CardTitle>{isAzure ? "Project - Azure access (Entra group + RBAC)" : isGcp ? "Project - GCP access (Entra group via WIF)" : "Project - AWS access (Identity Center)"}</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Name">
            <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} placeholder="team-alpha" required />
          </Field>
          <Field label="Environment">
            <select className={inputCls} value={environment} onChange={(e) => setEnvironment(e.target.value)}>
              <option value="dev">dev</option>
              <option value="staging">staging</option>
              <option value="production">production</option>
            </select>
          </Field>
          <Field label="Provider" hint={isAzure ? "Azure subscription whose access is vended." : isGcp ? "GCP project (Entra users via Workforce Identity Federation)." : "AWS Identity Center management/delegated-admin account."}>
            <select className={inputCls} value={provider} onChange={(e) => setProvider(e.target.value)} required>
              {providers.length === 0 && <option value="">no providers registered</option>}
              {(accessProviders.length > 0 ? accessProviders : providers).map((p) => (
                <option key={p.id} value={p.name}>
                  {p.name} ({p.type})
                </option>
              ))}
            </select>
          </Field>

          {isAzure ? (
            <>
              <Field label="Role" hint="Azure RBAC role granted to the group.">
                <select className={inputCls} value={roleName} onChange={(e) => setRoleName(e.target.value)} required>
                  <option value="Reader">Reader</option>
                  <option value="Contributor">Contributor</option>
                  <option value="Owner">Owner</option>
                  <option value="Storage Blob Data Reader">Storage Blob Data Reader</option>
                  <option value="Key Vault Secrets User">Key Vault Secrets User</option>
                </select>
              </Field>
              <Field label="Members" hint="Comma-separated Entra UPNs/emails (can add later).">
                <input className={inputCls} value={userNames} onChange={(e) => setUserNames(e.target.value)} placeholder="alice@contoso.com, bob@contoso.com" />
              </Field>
              <DeployIntoField
                provider={provider}
                providerType={selectedType}
                value={subscriptionId}
                onChange={setSubscriptionId}
                label="Target subscription"
                hint="Grant the team a role on a OPORD-managed subscription (from the subscription factory) - pick it here, no portal self-grant. Empty = the provider's default subscription."
              />
              <Field label="Resource group" hint="Optional - narrows the scope from the whole subscription to one RG.">
                <input className={inputCls} value={resourceGroup} onChange={(e) => setResourceGroup(e.target.value)} placeholder="(whole subscription)" />
              </Field>
              <label className="flex items-center gap-2 text-sm sm:col-span-2">
                <input type="checkbox" checked={pimEligible} onChange={(e) => setPimEligible(e.target.checked)} />
                <span>PIM-eligible (just-in-time) - members activate the role on demand. Requires Microsoft Entra ID P2.</span>
              </label>
            </>
          ) : isGcp ? (
            <>
              <Field label="Role" hint="IAM role granted on the project.">
                <select className={inputCls} value={gcpRole} onChange={(e) => setGcpRole(e.target.value)} required>
                  <option value="roles/viewer">roles/viewer</option>
                  <option value="roles/editor">roles/editor</option>
                  <option value="roles/owner">roles/owner</option>
                  <option value="roles/compute.viewer">roles/compute.viewer</option>
                  <option value="roles/storage.objectViewer">roles/storage.objectViewer</option>
                </select>
              </Field>
              <Field label="Entra group IDs" hint="Comma-separated Entra group object ids (GUIDs) - granted via the WIF principalSet.">
                <input className={inputCls} value={entraGroupIds} onChange={(e) => setEntraGroupIds(e.target.value)} placeholder="11111111-2222-3333-4444-555555555555" />
              </Field>
              <DeployIntoField
                provider={provider}
                providerType={selectedType}
                value={accountId}
                onChange={setAccountId}
                label="Target project"
                hint="Grant the team access to a OPORD-managed project (from the account factory) - pick it here, no console self-grant. Empty = the provider's default project."
              />
              <Field label="Workforce pool ID" hint="Optional - defaults to the provider config's workforce_pool_id.">
                <input className={inputCls} value={workforcePoolId} onChange={(e) => setWorkforcePoolId(e.target.value)} placeholder="opord-entra" />
              </Field>
              <Field label="Members (optional)" hint="Comma-separated bare Google emails to also grant (non-federated).">
                <input className={inputCls} value={userNames} onChange={(e) => setUserNames(e.target.value)} placeholder="(WIF groups above)" />
              </Field>
            </>
          ) : (
            <>
              <Field label="Target account ID" hint="12-digit existing account the group is granted access to.">
                <input className={inputCls} value={accountId} onChange={(e) => setAccountId(e.target.value)} placeholder="111122223333" required />
              </Field>
              <Field label="Members" hint="Comma-separated Identity Center usernames (can add later).">
                <input className={inputCls} value={userNames} onChange={(e) => setUserNames(e.target.value)} placeholder="alice@example.com, bob@example.com" />
              </Field>
              <Field label="Permission set name" hint="Created with the managed policies below.">
                <input className={inputCls} value={permissionSetName} onChange={(e) => setPermissionSetName(e.target.value)} placeholder="team-readonly" />
              </Field>
              <Field label="Managed policy ARNs" hint="Comma-separated.">
                <input className={inputCls} value={managedPolicyArns} onChange={(e) => setManagedPolicyArns(e.target.value)} />
              </Field>
            </>
          )}
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <button type="submit" disabled={submitting} className={cn(button({ size: "md" }), submitting && "opacity-70")}>
          {submitting && <Loader2 className="size-4 animate-spin" />}
          Create project
        </button>
        <Link href="/projects" className={button({ variant: "outline", size: "md" })}>
          Cancel
        </Link>
      </div>
    </form>
  );
}
