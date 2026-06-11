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

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-xs font-medium text-muted-foreground">{label}</span>
      {children}
      {hint && <span className="text-[11px] text-muted-foreground">{hint}</span>}
    </label>
  );
}

export function NewFunctionForm({ providers, initialProvider }: { providers: Provider[]; initialProvider?: string }) {
  const router = useRouter();
  const preset = initialProvider && providers.some((p) => p.name === initialProvider) ? initialProvider : undefined;

  const [name, setName] = useState("");
  const [environment, setEnvironment] = useState("dev");
  const [provider, setProvider] = useState(preset ?? providers.find((p) => p.type === "aws")?.name ?? providers[0]?.name ?? "");
  const [runtime, setRuntime] = useState("python3.12");
  const [region, setRegion] = useState("");
  const [memoryMb, setMemoryMb] = useState(128);
  const [timeoutSec, setTimeoutSec] = useState(10);
  const [ttlHours, setTtlHours] = useState(0);

  const [targetAccount, setTargetAccount] = useState("");

  const selectedType = providers.find((p) => p.name === provider)?.type;
  const isAws = selectedType === "aws";
  const isGCP = selectedType === "gcp";
  const isAzure = selectedType === "azure";

  // When the provider changes, reset the target account so a value from one
  // cloud doesn't leak into another. (The managed-account fetch + dropdown live in
  // <DeployIntoField/>.)
  const [prevProvider, setPrevProvider] = useState(provider);
  if (provider !== prevProvider) {
    setPrevProvider(provider);
    setTargetAccount("");
    setRegion("");
  }

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
        runtime,
        memory_mb: memoryMb,
        timeout_sec: timeoutSec,
        ttl_hours: ttlHours,
      };
      // GCP Cloud Functions honor a per-function region (else it lands in the
      // provider's configured region regardless of the selection).
      if (isGCP && region) spec.region = region;
      // Deploy into a OPORD-managed account (ADR-0013) - GCP project / Azure subscription /
      // AWS member account (cross-account AssumeRole).
      if ((isAws || isGCP || isAzure) && targetAccount.trim()) spec.target_account = targetAccount.trim();

      const res = await fetch(`${API}/api/v1/functions`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ name, environment, provider, spec }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `request failed (${res.status})`);
        return;
      }
      setOk(`Function "${data.name}" created - status ${data.status}.`);
      router.refresh();
      setTimeout(() => router.push("/functions"), 900);
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
          <CardTitle>{isAzure ? "Function (Azure Functions)" : isGCP ? "Function (Cloud Functions)" : "Function (AWS Lambda)"}</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Name">
            <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} placeholder="hello-fn" required />
          </Field>
          <Field label="Environment">
            <select className={inputCls} value={environment} onChange={(e) => setEnvironment(e.target.value)}>
              <option value="dev">dev</option>
              <option value="staging">staging</option>
              <option value="production">production</option>
            </select>
          </Field>
          <Field label="Provider" hint="AWS Lambda, GCP Cloud Functions, or Azure Functions.">
            <select className={inputCls} value={provider} onChange={(e) => setProvider(e.target.value)} required>
              {providers.length === 0 && <option value="">no providers registered</option>}
              {providers.map((p) => (
                <option key={p.id} value={p.name}>
                  {p.name} ({p.type})
                </option>
              ))}
            </select>
          </Field>
          <DeployIntoField provider={provider} providerType={selectedType} value={targetAccount} onChange={setTargetAccount} />
          <Field label="Runtime" hint={isAzure ? "e.g. python3.12, node20." : isGCP ? "e.g. python312, nodejs20." : "No code = built-in python handler (Lambda)."}>
            <select className={inputCls} value={runtime} onChange={(e) => setRuntime(e.target.value)}>
              <option value="python3.12">python3.12</option>
              <option value="python3.11">python3.11</option>
              <option value="nodejs20.x">nodejs20.x</option>
              <option value="nodejs18.x">nodejs18.x</option>
            </select>
          </Field>
          {isGCP && (
            <Field label="Region" hint="Empty = the provider's region. A governed project may restrict allowed locations.">
              <select className={inputCls} value={region} onChange={(e) => setRegion(e.target.value)}>
                <option value="">(provider default)</option>
                <option value="europe-west1">europe-west1</option>
                <option value="europe-west4">europe-west4</option>
                <option value="europe-north1">europe-north1</option>
                <option value="us-central1">us-central1</option>
                <option value="us-east1">us-east1</option>
                <option value="asia-east1">asia-east1</option>
              </select>
            </Field>
          )}
          <Field label="Memory (MB)">
            <input type="number" min={128} max={10240} step={64} className={inputCls} value={memoryMb} onChange={(e) => setMemoryMb(Number(e.target.value))} />
          </Field>
          <Field label="Timeout (s)">
            <input type="number" min={1} max={900} className={inputCls} value={timeoutSec} onChange={(e) => setTimeoutSec(Number(e.target.value))} />
          </Field>
          <Field label="TTL (hours)" hint="0 = never auto-destroy.">
            <input type="number" min={0} className={inputCls} value={ttlHours} onChange={(e) => setTtlHours(Number(e.target.value))} />
          </Field>
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <button type="submit" disabled={submitting} className={cn(button({ size: "md" }), submitting && "opacity-70")}>
          {submitting && <Loader2 className="size-4 animate-spin" />}
          {isAzure ? "Create Azure Function" : isGCP ? "Create Cloud Function" : "Create Lambda"}
        </button>
        <Link href="/functions" className={button({ variant: "outline", size: "md" })}>
          Cancel
        </Link>
      </div>
    </form>
  );
}
