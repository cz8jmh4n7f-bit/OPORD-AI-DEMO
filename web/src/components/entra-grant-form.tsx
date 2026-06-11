"use client";

import { useState } from "react";
import { Check, Loader2, TriangleAlert } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { authHeaders } from "@/lib/client-auth";

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

type GrantResult = { appRoleId: string; assigned: string[] | null; invited: string[] | null };

export function EntraGrantForm() {
  const [appId, setAppId] = useState("");
  const [roleArn, setRoleArn] = useState("");
  const [providerArn, setProviderArn] = useState("");
  const [roleName, setRoleName] = useState("ReadOnly");
  const [users, setUsers] = useState("");
  const [invite, setInvite] = useState(false);

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<GrantResult | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setResult(null);
    setSubmitting(true);
    try {
      const res = await fetch(`${API}/api/v1/entra/grant`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({
          appId,
          roleArn,
          providerArn,
          roleName,
          users: users
            .split(/[,\s]+/)
            .map((u) => u.trim())
            .filter(Boolean),
          invite,
        }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `request failed (${res.status})`);
        return;
      }
      setResult(data as GrantResult);
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
          <span className="break-all">{error}</span>
        </div>
      )}
      {result && (
        <div className="flex items-start gap-2 rounded-xl border border-success/30 bg-success/10 p-3 text-sm text-success">
          <Check className="mt-0.5 size-4 shrink-0" />
          <span>
            App role <code className="font-mono">{roleName}</code> ensured
            {result.appRoleId ? ` (id ${result.appRoleId})` : ""}.{" "}
            {result.invited?.length ? `Invited: ${result.invited.join(", ")}. ` : ""}
            Assigned: {result.assigned?.length ? result.assigned.join(", ") : "-"}. Users can now SSO and assume the role
            in AWS.
          </span>
        </div>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Grant SAML access (Entra to AWS)</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field
            label="Enterprise app (Application/client ID)"
            hint="In Azure: Enterprise applications, your app, Overview, Application ID."
          >
            <input
              className={inputCls}
              value={appId}
              onChange={(e) => setAppId(e.target.value)}
              placeholder="00000000-0000-0000-0000-000000000000"
              required
            />
          </Field>
          <Field label="Role name" hint="Label for the app role (Admin / Manager / ReadOnly / …).">
            <input className={inputCls} value={roleName} onChange={(e) => setRoleName(e.target.value)} placeholder="ReadOnly" required />
          </Field>
          <Field label="AWS role ARN" hint="The IAM role users assume.">
            <input
              className={inputCls}
              value={roleArn}
              onChange={(e) => setRoleArn(e.target.value)}
              placeholder="arn:aws:iam::123456789012:role/opord-<csa>-ReadOnly"
              required
            />
          </Field>
          <Field label="SAML provider ARN" hint="The IAM SAML provider created by L3 / the saml-federation stack.">
            <input
              className={inputCls}
              value={providerArn}
              onChange={(e) => setProviderArn(e.target.value)}
              placeholder="arn:aws:iam::123456789012:saml-provider/opord-<csa>-azuread"
              required
            />
          </Field>
          <Field label="Users (emails / UPNs)" hint="Comma- or space-separated.">
            <input
              className={inputCls}
              value={users}
              onChange={(e) => setUsers(e.target.value)}
              placeholder="alice@corp.com, bob@corp.com"
              required
            />
          </Field>
          <label className="flex items-center gap-2 self-end pb-2">
            <input type="checkbox" checked={invite} onChange={(e) => setInvite(e.target.checked)} className="size-4 rounded border-border" />
            <span className="text-sm text-foreground">Invite as B2B guests first (external emails)</span>
          </label>
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <button type="submit" disabled={submitting} className={cn(button({ size: "md" }), submitting && "opacity-70")}>
          {submitting && <Loader2 className="size-4 animate-spin" />}
          Grant access
        </button>
      </div>
    </form>
  );
}
