"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Check, Globe, Loader2, TriangleAlert } from "lucide-react";
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

export function NewDNSForm({ providers, initialProvider }: { providers: Provider[]; initialProvider?: string }) {
  const router = useRouter();
  // DNS zones (Route 53) are AWS-only - only the AWS provider implements them.
  const dnsProviders = providers.filter((p) => p.type === "aws");
  const preset = initialProvider && dnsProviders.some((p) => p.name === initialProvider) ? initialProvider : undefined;

  const [name, setName] = useState("");
  const [domain, setDomain] = useState("");
  const [environment, setEnvironment] = useState("dev");
  const [provider, setProvider] = useState(preset ?? dnsProviders[0]?.name ?? "");
  const [isPrivate, setIsPrivate] = useState(false);
  const [vpcId, setVpcId] = useState("");

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
        name: domain.trim(),
        private: isPrivate,
      };
      if (isPrivate && vpcId.trim()) spec.vpc_id = vpcId.trim();

      const res = await fetch(`${API}/api/v1/dns`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ name, environment, provider, spec }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `request failed (${res.status})`);
        return;
      }
      setOk(`DNS zone "${data.name}" created - status ${data.status}.`);
      router.refresh();
      setTimeout(() => router.push("/dns"), 900);
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
          <CardTitle>Hosted zone (Route 53)</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Resource name" hint="OPORD tracking name.">
            <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} placeholder="example-com" required />
          </Field>
          <Field label="Domain" hint="The DNS domain for the zone, e.g. example.com.">
            <input className={inputCls} value={domain} onChange={(e) => setDomain(e.target.value)} placeholder="example.com" required />
          </Field>
          <Field label="Environment">
            <select className={inputCls} value={environment} onChange={(e) => setEnvironment(e.target.value)}>
              <option value="dev">dev</option>
              <option value="staging">staging</option>
              <option value="production">production</option>
            </select>
          </Field>
          <Field label="Provider" hint="AWS (Route 53).">
            <select className={inputCls} value={provider} onChange={(e) => setProvider(e.target.value)} required>
              {dnsProviders.length === 0 && <option value="">no AWS providers registered</option>}
              {dnsProviders.map((p) => (
                <option key={p.id} value={p.name}>
                  {p.name} ({p.type})
                </option>
              ))}
            </select>
          </Field>
          <Field label="Visibility" hint="Private zones resolve only inside an associated VPC.">
            <label className="flex h-9 items-center gap-2 text-sm text-foreground">
              <input type="checkbox" checked={isPrivate} onChange={(e) => setIsPrivate(e.target.checked)} />
              Private (VPC) zone
            </label>
          </Field>
          {isPrivate && (
            <Field label="VPC ID" hint="The VPC to associate the private zone with.">
              <input className={inputCls} value={vpcId} onChange={(e) => setVpcId(e.target.value)} placeholder="vpc-0123456789abcdef0" />
            </Field>
          )}
          <div className="flex items-start gap-3 rounded-lg border border-border bg-muted/40 p-3 sm:col-span-2">
            <Globe className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
            <div className="text-sm text-muted-foreground">
              A Route 53 hosted zone is created for the domain. Public zones return name servers to delegate to at your
              registrar; private zones resolve only within the associated VPC.
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <button type="submit" disabled={submitting} className={cn(button({ size: "md" }), submitting && "opacity-70")}>
          {submitting && <Loader2 className="size-4 animate-spin" />}
          Create zone
        </button>
        <Link href="/dns" className={button({ variant: "outline", size: "md" })}>
          Cancel
        </Link>
      </div>
    </form>
  );
}
