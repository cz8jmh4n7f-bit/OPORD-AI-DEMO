"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { BadgeCheck, Check, Loader2, TriangleAlert } from "lucide-react";
import type { Provider } from "@/lib/types";
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

// Split a comma-separated input into a trimmed, non-empty string array.
function csv(v: string): string[] {
  return v
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);
}

export function NewCertForm({ providers, initialProvider }: { providers: Provider[]; initialProvider?: string }) {
  const router = useRouter();
  // TLS certificates (ACM) are AWS-only - only the AWS provider implements them.
  const certProviders = providers.filter((p) => p.type === "aws");
  const preset = initialProvider && certProviders.some((p) => p.name === initialProvider) ? initialProvider : undefined;

  const [name, setName] = useState("");
  const [domain, setDomain] = useState("");
  const [environment, setEnvironment] = useState("dev");
  const [provider, setProvider] = useState(preset ?? certProviders[0]?.name ?? "");
  const [sans, setSans] = useState("");
  const [validationZoneId, setValidationZoneId] = useState("");
  const [forCloudFront, setForCloudFront] = useState(false);

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setOk(null);
    setSubmitting(true);
    try {
      const spec: Record<string, unknown> = {
        domain: domain.trim(),
        for_cloudfront: forCloudFront,
      };
      const sanList = csv(sans);
      if (sanList.length > 0) spec.subject_alternative_names = sanList;
      if (validationZoneId.trim()) spec.validation_zone_id = validationZoneId.trim();

      const res = await fetch(`${API}/api/v1/certs`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ name, environment, provider, spec }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `request failed (${res.status})`);
        return;
      }
      setOk(`Certificate "${data.name}" created - status ${data.status}.`);
      router.refresh();
      setTimeout(() => router.push("/certs"), 900);
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
          <CardTitle>Certificate (ACM)</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Resource name" hint="OPORD tracking name.">
            <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} placeholder="app-cert" required />
          </Field>
          <Field label="Domain" hint="Primary domain, e.g. app.example.com.">
            <input className={inputCls} value={domain} onChange={(e) => setDomain(e.target.value)} placeholder="app.example.com" required />
          </Field>
          <Field label="Environment">
            <select className={inputCls} value={environment} onChange={(e) => setEnvironment(e.target.value)}>
              <option value="dev">dev</option>
              <option value="staging">staging</option>
              <option value="production">production</option>
            </select>
          </Field>
          <Field label="Provider" hint="AWS (ACM).">
            <select className={inputCls} value={provider} onChange={(e) => setProvider(e.target.value)} required>
              {certProviders.length === 0 && <option value="">no AWS providers registered</option>}
              {certProviders.map((p) => (
                <option key={p.id} value={p.name}>
                  {p.name} ({p.type})
                </option>
              ))}
            </select>
          </Field>
          <Field label="Subject alternative names" hint="Comma-separated extra domains (SANs). Optional.">
            <input className={inputCls} value={sans} onChange={(e) => setSans(e.target.value)} placeholder="www.example.com, api.example.com" />
          </Field>
          <Field label="Validation zone ID" hint="Route 53 hosted zone ID for automatic DNS validation. Optional.">
            <input className={inputCls} value={validationZoneId} onChange={(e) => setValidationZoneId(e.target.value)} placeholder="Z0123456789ABCDEFGHIJ" />
          </Field>
          <Field label="For CloudFront" hint="CloudFront requires the certificate in us-east-1.">
            <label className="flex h-9 items-center gap-2 text-sm text-foreground">
              <input type="checkbox" checked={forCloudFront} onChange={(e) => setForCloudFront(e.target.checked)} />
              Issue in us-east-1 for CloudFront
            </label>
          </Field>
          <div className="flex items-start gap-3 rounded-lg border border-border bg-muted/40 p-3 sm:col-span-2">
            <BadgeCheck className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
            <div className="text-sm text-muted-foreground">
              An ACM certificate is requested for the domain (and any SANs). With a validation zone ID, OPORD creates the
              DNS validation records automatically so the certificate issues without manual steps.
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <button type="submit" disabled={submitting} className={cn(button({ size: "md" }), submitting && "opacity-70")}>
          {submitting && <Loader2 className="size-4 animate-spin" />}
          Create certificate
        </button>
        <Link href="/certs" className={button({ variant: "outline", size: "md" })}>
          Cancel
        </Link>
      </div>
    </form>
  );
}
