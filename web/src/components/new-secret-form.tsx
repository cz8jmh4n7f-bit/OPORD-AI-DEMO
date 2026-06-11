"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { KeyRound, Check, Loader2, TriangleAlert } from "lucide-react";
import type { Provider } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { authHeaders } from "@/lib/client-auth";
import { DeployIntoField } from "@/components/deploy-into-field";

const API = "/bff";
const inputCls =
  "h-9 w-full rounded-lg border border-input bg-card px-3 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-xs font-medium text-muted-foreground">{label}</span>
      {children}
      {hint && <span className="text-[11px] text-muted-foreground">{hint}</span>}
    </label>
  );
}

export function NewSecretForm({ providers, initialProvider }: { providers: Provider[]; initialProvider?: string }) {
  const router = useRouter();
  // Managed secrets work on AWS (Secrets Manager), Azure (Key Vault) and GCP
  // (Secret Manager) - all three implement the SecretProvisioner capability.
  const secretProviders = providers.filter((p) => p.type === "aws" || p.type === "azure" || p.type === "gcp");
  const preset = initialProvider && providers.some((p) => p.name === initialProvider) ? initialProvider : undefined;

  const [name, setName] = useState("");
  const [secretName, setSecretName] = useState("");
  const [environment, setEnvironment] = useState("dev");
  const [provider, setProvider] = useState(preset ?? secretProviders[0]?.name ?? providers[0]?.name ?? "");
  const [description, setDescription] = useState("");

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);

  const [targetAccount, setTargetAccount] = useState("");

  const selectedType = providers.find((p) => p.name === provider)?.type;
  const isAws = selectedType === "aws";
  const isAzure = selectedType === "azure";
  const isGCP = selectedType === "gcp";

  // When the provider changes, reset the target account so a value from one
  // cloud doesn't leak into another. (The managed-account fetch + dropdown live in
  // <DeployIntoField/>.)
  const [prevProvider, setPrevProvider] = useState(provider);
  if (provider !== prevProvider) {
    setPrevProvider(provider);
    setTargetAccount("");
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setOk(null);
    setSubmitting(true);
    try {
      const spec: Record<string, unknown> = {
        name: secretName.trim() || name.trim(),
      };
      // Description is AWS-only (Azure Key Vault / GCP Secret Manager have no description here).
      if (!isAzure && !isGCP && description.trim()) spec.description = description.trim();
      // Deploy into a OPORD-managed account (ADR-0013) instead of the provider's
      // default - GCP project / Azure subscription / AWS member account (cross-account AssumeRole).
      if ((isAws || isGCP || isAzure) && targetAccount.trim()) spec.target_account = targetAccount.trim();

      const res = await fetch(`${API}/api/v1/secrets`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ name, environment, provider, spec }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `request failed (${res.status})`);
        return;
      }
      setOk(`Secret "${data.name}" created - status ${data.status}.`);
      router.refresh();
      setTimeout(() => router.push("/secrets"), 900);
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
          <CardTitle>{isAzure ? "Secret (Azure Key Vault)" : isGCP ? "Secret (GCP Secret Manager)" : "Secret (AWS Secrets Manager)"}</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Resource name" hint="OPORD tracking name.">
            <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} placeholder="jwt-key" required />
          </Field>
          <Field label={isAzure ? "Vault name prefix" : isGCP ? "Secret name" : "Secret name / path"} hint="Optional. Defaults to the resource name.">
            <input className={inputCls} value={secretName} onChange={(e) => setSecretName(e.target.value)} placeholder={isAzure ? "opordapp" : isGCP ? "prod-api-jwt-key" : "prod/api/jwt-key"} />
          </Field>
          <Field label="Environment">
            <select className={inputCls} value={environment} onChange={(e) => setEnvironment(e.target.value)}>
              <option value="dev">dev</option>
              <option value="staging">staging</option>
              <option value="production">production</option>
            </select>
          </Field>
          <Field label="Provider" hint="AWS (Secrets Manager), Azure (Key Vault) or GCP (Secret Manager).">
            <select className={inputCls} value={provider} onChange={(e) => setProvider(e.target.value)} required>
              {providers.length === 0 && <option value="">no providers registered</option>}
              {(secretProviders.length > 0 ? secretProviders : providers).map((p) => (
                <option key={p.id} value={p.name}>
                  {p.name} ({p.type})
                </option>
              ))}
            </select>
          </Field>
          {!isAzure && !isGCP && (
            <Field label="Description" hint="Optional. Shown in the AWS console + audit.">
              <input className={inputCls} value={description} onChange={(e) => setDescription(e.target.value)} placeholder="JWT signing key" />
            </Field>
          )}
          <DeployIntoField provider={provider} providerType={selectedType} value={targetAccount} onChange={setTargetAccount} />
          <div className="flex items-start gap-3 rounded-lg border border-border bg-muted/40 p-3 sm:col-span-2">
            <KeyRound className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
            <div className="text-sm text-muted-foreground">
              OPORD provisions the secret container only - the plaintext value is set out-of-band (console / CLI / Vault-sync), so OPORD never holds your credentials.
              {isAzure
                ? " The Key Vault is created with purge protection off so it can be destroyed cleanly in dev."
                : isGCP
                  ? " The Secret Manager secret container is provisioned; the value is set out-of-band."
                  : ""}
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <button type="submit" disabled={submitting} className={cn(button({ size: "md" }), submitting && "opacity-70")}>
          {submitting && <Loader2 className="size-4 animate-spin" />}
          {isAzure ? "Create Key Vault" : isGCP ? "Create secret" : "Create secret"}
        </button>
        <Link href="/secrets" className={button({ variant: "outline", size: "md" })}>
          Cancel
        </Link>
      </div>
    </form>
  );
}
