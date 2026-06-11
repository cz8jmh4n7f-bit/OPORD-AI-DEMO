"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Gauge, Check, Loader2, TriangleAlert } from "lucide-react";
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

export function NewCacheForm({ providers, initialProvider }: { providers: Provider[]; initialProvider?: string }) {
  const router = useRouter();
  // Caches work on AWS (ElastiCache), Azure (Cache for Redis), and GCP
  // (Memorystore for Redis) - all implement the CacheProvisioner capability.
  const cacheProviders = providers.filter((p) => p.type === "aws" || p.type === "azure" || p.type === "gcp");
  const preset = initialProvider && providers.some((p) => p.name === initialProvider) ? initialProvider : undefined;

  const [name, setName] = useState("");
  const [cacheName, setCacheName] = useState("");
  const [environment, setEnvironment] = useState("dev");
  const [provider, setProvider] = useState(preset ?? cacheProviders[0]?.name ?? providers[0]?.name ?? "");
  const [highAvailability, setHighAvailability] = useState(false);
  const [nodeType, setNodeType] = useState("");

  const [targetAccount, setTargetAccount] = useState("");

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);

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
        name: cacheName.trim() || name.trim(),
        engine_version: "7.1",
        // HA: >1 node enables ElastiCache Multi-AZ / the Azure Standard SKU.
        num_cache_nodes: highAvailability ? 2 : 1,
        at_rest_encryption: true,
        in_transit_encryption: true,
      };
      // node_type (cache.* class) is an AWS concept; Azure/GCP pick their own SKU.
      if (!isAzure && !isGCP && nodeType.trim()) spec.node_type = nodeType.trim();
      // Deploy into a OPORD-managed account (ADR-0013) - GCP project / Azure subscription /
      // AWS member account (cross-account AssumeRole).
      if ((isAws || isGCP || isAzure) && targetAccount.trim()) spec.target_account = targetAccount.trim();

      const res = await fetch(`${API}/api/v1/caches`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ name, environment, provider, spec }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `request failed (${res.status})`);
        return;
      }
      setOk(`Cache "${data.name}" created - status ${data.status}.`);
      router.refresh();
      setTimeout(() => router.push("/caches"), 900);
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
          <CardTitle>{isAzure ? "Cache (Azure Cache for Redis)" : isGCP ? "Cache (GCP Memorystore for Redis)" : "Cache (AWS ElastiCache Redis)"}</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Resource name" hint="OPORD tracking name.">
            <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} placeholder="sessions" required />
          </Field>
          <Field label="Cache name" hint="Optional. Defaults to the resource name.">
            <input className={inputCls} value={cacheName} onChange={(e) => setCacheName(e.target.value)} placeholder="sessions" />
          </Field>
          <Field label="Environment">
            <select className={inputCls} value={environment} onChange={(e) => setEnvironment(e.target.value)}>
              <option value="dev">dev</option>
              <option value="staging">staging</option>
              <option value="production">production</option>
            </select>
          </Field>
          <Field label="Provider" hint="AWS (ElastiCache), Azure (Redis), or GCP (Memorystore).">
            <select className={inputCls} value={provider} onChange={(e) => setProvider(e.target.value)} required>
              {providers.length === 0 && <option value="">no providers registered</option>}
              {(cacheProviders.length > 0 ? cacheProviders : providers).map((p) => (
                <option key={p.id} value={p.name}>
                  {p.name} ({p.type})
                </option>
              ))}
            </select>
          </Field>
          {!isAzure && !isGCP && (
            <Field label="Node type" hint="Optional ElastiCache class, e.g. cache.t4g.micro. Defaults to the module default.">
              <input className={inputCls} value={nodeType} onChange={(e) => setNodeType(e.target.value)} placeholder="cache.t4g.micro" />
            </Field>
          )}
          <DeployIntoField provider={provider} providerType={selectedType} value={targetAccount} onChange={setTargetAccount} />
          <label className="flex items-center gap-2 text-sm sm:col-span-2">
            <input type="checkbox" checked={highAvailability} onChange={(e) => setHighAvailability(e.target.checked)} />
            <span>High availability {isAzure ? "(Standard SKU, replicated)" : isGCP ? "(STANDARD_HA tier, read replica)" : "(Multi-AZ + automatic failover)"}</span>
          </label>
          <div className="flex items-start gap-3 rounded-lg border border-border bg-muted/40 p-3 sm:col-span-2">
            <Gauge className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
            <div className="text-sm text-muted-foreground">
              {isAzure
                ? "An Azure Cache for Redis is created in its own resource group, TLS-only (non-SSL port off). Default Basic C0 (250MB); HA upgrades to the replicated Standard SKU. Access keys are read from Azure, never persisted."
                : isGCP
                  ? "A Memorystore for Redis instance in the provider's region. BASIC tier by default; HA uses STANDARD_HA with a read replica. Transit encryption enabled."
                  : "An ElastiCache Redis replication group with in-transit + at-rest encryption. Needs private subnets (subnet_ids) from the provider config. HA spans 2 AZs with automatic failover."}
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <button type="submit" disabled={submitting} className={cn(button({ size: "md" }), submitting && "opacity-70")}>
          {submitting && <Loader2 className="size-4 animate-spin" />}
          {isAzure ? "Create Azure Cache for Redis" : isGCP ? "Create Memorystore instance" : "Create ElastiCache cluster"}
        </button>
        <Link href="/caches" className={button({ variant: "outline", size: "md" })}>
          Cancel
        </Link>
      </div>
    </form>
  );
}
