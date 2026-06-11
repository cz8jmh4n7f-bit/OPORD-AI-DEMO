"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Check, Loader2, Network, TriangleAlert } from "lucide-react";
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

// Split a comma-separated input into a trimmed, non-empty string array.
function csv(v: string): string[] {
  return v
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);
}

export function NewLoadBalancerForm({ providers, initialProvider }: { providers: Provider[]; initialProvider?: string }) {
  const router = useRouter();
  // Load balancers (ALB) are AWS-only - only the AWS provider implements them.
  const lbProviders = providers.filter((p) => p.type === "aws");
  const preset = initialProvider && lbProviders.some((p) => p.name === initialProvider) ? initialProvider : undefined;

  const [name, setName] = useState("");
  const [environment, setEnvironment] = useState("dev");
  const [provider, setProvider] = useState(preset ?? lbProviders[0]?.name ?? "");
  const [internal, setInternal] = useState(false);
  const [subnetIds, setSubnetIds] = useState("");
  const [targetType, setTargetType] = useState("instance");
  const [targets, setTargets] = useState("");
  const [healthCheckPath, setHealthCheckPath] = useState("/");

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);

  const selectedType = providers.find((p) => p.name === provider)?.type;
  const isAws = selectedType === "aws";

  const [targetAccount, setTargetAccount] = useState("");

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
        internal,
        target_type: targetType,
        health_check_path: healthCheckPath.trim() || "/",
      };
      const subnetList = csv(subnetIds);
      if (subnetList.length > 0) spec.subnet_ids = subnetList;
      const targetList = csv(targets);
      if (targetList.length > 0) spec.targets = targetList;
      // Deploy into a OPORD-managed account (ADR-0013) - AWS member account
      // (cross-account AssumeRole).
      if (isAws && targetAccount.trim()) spec.target_account = targetAccount.trim();

      const res = await fetch(`${API}/api/v1/loadbalancers`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ name, environment, provider, spec }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `request failed (${res.status})`);
        return;
      }
      setOk(`Load balancer "${data.name}" created - status ${data.status}.`);
      router.refresh();
      setTimeout(() => router.push("/loadbalancers"), 900);
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
          <CardTitle>Load balancer (ALB)</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Resource name" hint="OPORD tracking name.">
            <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} placeholder="web-alb" required />
          </Field>
          <Field label="Environment">
            <select className={inputCls} value={environment} onChange={(e) => setEnvironment(e.target.value)}>
              <option value="dev">dev</option>
              <option value="staging">staging</option>
              <option value="production">production</option>
            </select>
          </Field>
          <Field label="Provider" hint="AWS (ALB).">
            <select className={inputCls} value={provider} onChange={(e) => setProvider(e.target.value)} required>
              {lbProviders.length === 0 && <option value="">no AWS providers registered</option>}
              {lbProviders.map((p) => (
                <option key={p.id} value={p.name}>
                  {p.name} ({p.type})
                </option>
              ))}
            </select>
          </Field>
          <DeployIntoField provider={provider} providerType={selectedType} value={targetAccount} onChange={setTargetAccount} />
          <Field label="Scheme" hint="Internal load balancers are reachable only from within the VPC.">
            <label className="flex h-9 items-center gap-2 text-sm text-foreground">
              <input type="checkbox" checked={internal} onChange={(e) => setInternal(e.target.checked)} />
              Internal (not internet-facing)
            </label>
          </Field>
          <Field label="Subnet IDs" hint="Comma-separated (≥2 AZs). Empty uses the provider config subnets.">
            <input className={inputCls} value={subnetIds} onChange={(e) => setSubnetIds(e.target.value)} placeholder="subnet-aaa, subnet-bbb" />
          </Field>
          <Field label="Target type">
            <select className={inputCls} value={targetType} onChange={(e) => setTargetType(e.target.value)}>
              <option value="instance">instance</option>
              <option value="ip">ip</option>
              <option value="lambda">lambda</option>
            </select>
          </Field>
          <Field label="Targets" hint="Comma-separated instance IDs / IPs / a Lambda ARN. Optional.">
            <input className={inputCls} value={targets} onChange={(e) => setTargets(e.target.value)} placeholder="i-0123, i-0456" />
          </Field>
          <Field label="Health check path" hint="HTTP path the target group probes.">
            <input className={inputCls} value={healthCheckPath} onChange={(e) => setHealthCheckPath(e.target.value)} placeholder="/" />
          </Field>
          <div className="flex items-start gap-3 rounded-lg border border-border bg-muted/40 p-3 sm:col-span-2">
            <Network className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
            <div className="text-sm text-muted-foreground">
              An application load balancer is created with an HTTP:80 listener and a target group. With no subnets given,
              OPORD uses the provider config subnets; an auto VPC-CIDR-scoped security group is created when none is set.
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <button type="submit" disabled={submitting} className={cn(button({ size: "md" }), submitting && "opacity-70")}>
          {submitting && <Loader2 className="size-4 animate-spin" />}
          Create load balancer
        </button>
        <Link href="/loadbalancers" className={button({ variant: "outline", size: "md" })}>
          Cancel
        </Link>
      </div>
    </form>
  );
}
