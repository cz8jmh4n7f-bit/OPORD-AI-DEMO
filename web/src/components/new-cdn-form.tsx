"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Check, Globe2, Loader2, TriangleAlert } from "lucide-react";
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

export function NewCDNForm({ providers, initialProvider }: { providers: Provider[]; initialProvider?: string }) {
  const router = useRouter();
  // CDNs (CloudFront) are AWS-only - only the AWS provider implements them.
  const cdnProviders = providers.filter((p) => p.type === "aws");
  const preset = initialProvider && cdnProviders.some((p) => p.name === initialProvider) ? initialProvider : undefined;

  const [name, setName] = useState("");
  const [environment, setEnvironment] = useState("dev");
  const [provider, setProvider] = useState(preset ?? cdnProviders[0]?.name ?? "");
  const [originType, setOriginType] = useState("s3");
  const [originDomain, setOriginDomain] = useState("");
  const [aliases, setAliases] = useState("");
  const [certificateArn, setCertificateArn] = useState("");
  const [defaultRootObject, setDefaultRootObject] = useState("");
  const [priceClass, setPriceClass] = useState("PriceClass_100");

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
        origin_type: originType,
        origin_domain: originDomain.trim(),
        price_class: priceClass,
      };
      const aliasList = csv(aliases);
      if (aliasList.length > 0) spec.aliases = aliasList;
      if (certificateArn.trim()) spec.certificate_arn = certificateArn.trim();
      if (defaultRootObject.trim()) spec.default_root_object = defaultRootObject.trim();

      const res = await fetch(`${API}/api/v1/cdns`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ name, environment, provider, spec }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `request failed (${res.status})`);
        return;
      }
      setOk(`CDN "${data.name}" created - status ${data.status}.`);
      router.refresh();
      setTimeout(() => router.push("/cdns"), 900);
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
          <CardTitle>CDN (CloudFront)</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Resource name" hint="OPORD tracking name.">
            <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} placeholder="assets-cdn" required />
          </Field>
          <Field label="Environment">
            <select className={inputCls} value={environment} onChange={(e) => setEnvironment(e.target.value)}>
              <option value="dev">dev</option>
              <option value="staging">staging</option>
              <option value="production">production</option>
            </select>
          </Field>
          <Field label="Provider" hint="AWS (CloudFront).">
            <select className={inputCls} value={provider} onChange={(e) => setProvider(e.target.value)} required>
              {cdnProviders.length === 0 && <option value="">no AWS providers registered</option>}
              {cdnProviders.map((p) => (
                <option key={p.id} value={p.name}>
                  {p.name} ({p.type})
                </option>
              ))}
            </select>
          </Field>
          <Field label="Origin type">
            <select className={inputCls} value={originType} onChange={(e) => setOriginType(e.target.value)}>
              <option value="s3">s3</option>
              <option value="alb">alb</option>
              <option value="apigw">apigw</option>
              <option value="custom">custom</option>
            </select>
          </Field>
          <Field label="Origin domain" hint="Bucket regional domain / ALB DNS / API endpoint / custom origin.">
            <input className={inputCls} value={originDomain} onChange={(e) => setOriginDomain(e.target.value)} placeholder="opord-assets.s3.amazonaws.com" required />
          </Field>
          <Field label="Aliases" hint="Comma-separated CNAMEs to serve. Need a us-east-1 certificate.">
            <input className={inputCls} value={aliases} onChange={(e) => setAliases(e.target.value)} placeholder="cdn.example.com" />
          </Field>
          <Field label="Certificate ARN" hint="Must be a us-east-1 (N. Virginia) ACM cert for the aliases.">
            <input className={inputCls} value={certificateArn} onChange={(e) => setCertificateArn(e.target.value)} placeholder="arn:aws:acm:us-east-1:..." />
          </Field>
          <Field label="Default root object" hint="Object served for the root path, e.g. index.html. Optional.">
            <input className={inputCls} value={defaultRootObject} onChange={(e) => setDefaultRootObject(e.target.value)} placeholder="index.html" />
          </Field>
          <Field label="Price class">
            <select className={inputCls} value={priceClass} onChange={(e) => setPriceClass(e.target.value)}>
              <option value="PriceClass_100">PriceClass_100</option>
              <option value="PriceClass_200">PriceClass_200</option>
              <option value="PriceClass_All">PriceClass_All</option>
            </select>
          </Field>
          <div className="flex items-start gap-3 rounded-lg border border-border bg-muted/40 p-3 sm:col-span-2">
            <Globe2 className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
            <div className="text-sm text-muted-foreground">
              A CloudFront distribution is created in front of the origin. To serve your own domain, set aliases plus a
              us-east-1 ACM certificate; the price class controls which edge locations are used.
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <button type="submit" disabled={submitting} className={cn(button({ size: "md" }), submitting && "opacity-70")}>
          {submitting && <Loader2 className="size-4 animate-spin" />}
          Create CDN
        </button>
        <Link href="/cdns" className={button({ variant: "outline", size: "md" })}>
          Cancel
        </Link>
      </div>
    </form>
  );
}
