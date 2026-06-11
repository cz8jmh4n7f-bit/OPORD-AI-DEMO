"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Check, Loader2, TriangleAlert, Webhook } from "lucide-react";
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

export function NewAPIGatewayForm({ providers, initialProvider }: { providers: Provider[]; initialProvider?: string }) {
  const router = useRouter();
  // HTTP APIs (API Gateway) are AWS-only - only the AWS provider implements them.
  const gwProviders = providers.filter((p) => p.type === "aws");
  const preset = initialProvider && gwProviders.some((p) => p.name === initialProvider) ? initialProvider : undefined;

  const [name, setName] = useState("");
  const [environment, setEnvironment] = useState("dev");
  const [provider, setProvider] = useState(preset ?? gwProviders[0]?.name ?? "");
  const [integrationType, setIntegrationType] = useState("lambda");
  const [integrationTarget, setIntegrationTarget] = useState("");
  const [routeKey, setRouteKey] = useState("$default");
  const [domainName, setDomainName] = useState("");
  const [certificateArn, setCertificateArn] = useState("");
  const [hostedZoneId, setHostedZoneId] = useState("");

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
        integration_type: integrationType,
        integration_target: integrationTarget.trim(),
        route_key: routeKey.trim() || "$default",
      };
      if (domainName.trim()) spec.domain_name = domainName.trim();
      if (certificateArn.trim()) spec.certificate_arn = certificateArn.trim();
      if (hostedZoneId.trim()) spec.hosted_zone_id = hostedZoneId.trim();

      const res = await fetch(`${API}/api/v1/apigateways`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ name, environment, provider, spec }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `request failed (${res.status})`);
        return;
      }
      setOk(`API gateway "${data.name}" created - status ${data.status}.`);
      router.refresh();
      setTimeout(() => router.push("/apigateways"), 900);
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
          <CardTitle>HTTP API (API Gateway)</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Resource name" hint="OPORD tracking name.">
            <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} placeholder="public-api" required />
          </Field>
          <Field label="Environment">
            <select className={inputCls} value={environment} onChange={(e) => setEnvironment(e.target.value)}>
              <option value="dev">dev</option>
              <option value="staging">staging</option>
              <option value="production">production</option>
            </select>
          </Field>
          <Field label="Provider" hint="AWS (API Gateway).">
            <select className={inputCls} value={provider} onChange={(e) => setProvider(e.target.value)} required>
              {gwProviders.length === 0 && <option value="">no AWS providers registered</option>}
              {gwProviders.map((p) => (
                <option key={p.id} value={p.name}>
                  {p.name} ({p.type})
                </option>
              ))}
            </select>
          </Field>
          <Field label="Integration type">
            <select className={inputCls} value={integrationType} onChange={(e) => setIntegrationType(e.target.value)}>
              <option value="lambda">lambda</option>
              <option value="http">http</option>
            </select>
          </Field>
          <Field label="Integration target" hint={integrationType === "lambda" ? "Lambda function ARN." : "Upstream HTTP URL."}>
            <input
              className={inputCls}
              value={integrationTarget}
              onChange={(e) => setIntegrationTarget(e.target.value)}
              placeholder={integrationType === "lambda" ? "arn:aws:lambda:..." : "https://upstream.example.com"}
              required
            />
          </Field>
          <Field label="Route key" hint='Default "$default" is a catch-all proxy.'>
            <input className={inputCls} value={routeKey} onChange={(e) => setRouteKey(e.target.value)} placeholder="$default" />
          </Field>
          <Field label="Custom domain" hint="Optional. Maps a custom domain to the API.">
            <input className={inputCls} value={domainName} onChange={(e) => setDomainName(e.target.value)} placeholder="api.example.com" />
          </Field>
          <Field label="Certificate ARN" hint="Required when a custom domain is set.">
            <input className={inputCls} value={certificateArn} onChange={(e) => setCertificateArn(e.target.value)} placeholder="arn:aws:acm:..." />
          </Field>
          <Field label="Hosted zone ID" hint="Route 53 zone for the custom domain DNS record. Optional.">
            <input className={inputCls} value={hostedZoneId} onChange={(e) => setHostedZoneId(e.target.value)} placeholder="Z0123456789ABCDEFGHIJ" />
          </Field>
          <div className="flex items-start gap-3 rounded-lg border border-border bg-muted/40 p-3 sm:col-span-2">
            <Webhook className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
            <div className="text-sm text-muted-foreground">
              An HTTP API is created with a default stage and a route to the integration target. Provide a custom domain
              plus an ACM certificate (and a hosted zone) to serve it on your own domain.
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <button type="submit" disabled={submitting} className={cn(button({ size: "md" }), submitting && "opacity-70")}>
          {submitting && <Loader2 className="size-4 animate-spin" />}
          Create API gateway
        </button>
        <Link href="/apigateways" className={button({ variant: "outline", size: "md" })}>
          Cancel
        </Link>
      </div>
    </form>
  );
}
