"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Inbox, Check, Loader2, TriangleAlert } from "lucide-react";
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

export function NewQueueForm({ providers, initialProvider }: { providers: Provider[]; initialProvider?: string }) {
  const router = useRouter();
  // Message queues work on AWS (SQS), Azure (Service Bus), and GCP (Pub/Sub) -
  // all three implement the QueueProvisioner capability.
  const queueProviders = providers.filter((p) => p.type === "aws" || p.type === "azure" || p.type === "gcp");
  const preset = initialProvider && providers.some((p) => p.name === initialProvider) ? initialProvider : undefined;

  const [name, setName] = useState("");
  const [queueName, setQueueName] = useState("");
  const [environment, setEnvironment] = useState("dev");
  const [provider, setProvider] = useState(preset ?? queueProviders[0]?.name ?? providers[0]?.name ?? "");
  const [fifo, setFifo] = useState(false);
  const [dlq, setDlq] = useState(false);

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);

  const selectedType = providers.find((p) => p.name === provider)?.type;
  const isAws = selectedType === "aws";
  const isAzure = selectedType === "azure";
  const isGCP = selectedType === "gcp";

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
        name: queueName.trim() || name.trim(),
        dlq_enabled: dlq,
      };
      // FIFO (.fifo suffix) is an SQS concept; Azure Service Bus queues are
      // ordered within a session and the module ignores this flag. GCP Pub/Sub
      // ordering is per-key - the flag is silently ignored there too.
      if (!isAzure && !isGCP) spec.fifo = fifo;
      // Deploy into a OPORD-managed account (ADR-0013) - GCP project / Azure subscription /
      // AWS member account (cross-account AssumeRole).
      if ((isAws || isGCP || isAzure) && targetAccount.trim()) spec.target_account = targetAccount.trim();

      const res = await fetch(`${API}/api/v1/queues`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ name, environment, provider, spec }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `request failed (${res.status})`);
        return;
      }
      setOk(`Queue "${data.name}" created - status ${data.status}.`);
      router.refresh();
      setTimeout(() => router.push("/queues"), 900);
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
          <CardTitle>{isAzure ? "Queue (Azure Service Bus)" : isGCP ? "Queue (GCP Pub/Sub)" : "Queue (AWS SQS)"}</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Resource name" hint="OPORD tracking name.">
            <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} placeholder="jobs" required />
          </Field>
          <Field label="Queue name" hint="Optional. Defaults to the resource name.">
            <input className={inputCls} value={queueName} onChange={(e) => setQueueName(e.target.value)} placeholder="jobs" />
          </Field>
          <Field label="Environment">
            <select className={inputCls} value={environment} onChange={(e) => setEnvironment(e.target.value)}>
              <option value="dev">dev</option>
              <option value="staging">staging</option>
              <option value="production">production</option>
            </select>
          </Field>
          <Field label="Provider" hint="AWS (SQS), Azure (Service Bus), or GCP (Pub/Sub).">
            <select className={inputCls} value={provider} onChange={(e) => setProvider(e.target.value)} required>
              {providers.length === 0 && <option value="">no providers registered</option>}
              {(queueProviders.length > 0 ? queueProviders : providers).map((p) => (
                <option key={p.id} value={p.name}>
                  {p.name} ({p.type})
                </option>
              ))}
            </select>
          </Field>
          {!isAzure && !isGCP && (
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={fifo} onChange={(e) => setFifo(e.target.checked)} />
              <span>FIFO ordering (exactly-once)</span>
            </label>
          )}
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={dlq} onChange={(e) => setDlq(e.target.checked)} />
            <span>Dead-letter queue {isAzure ? "(dead-lettering on)" : isGCP ? "(dead-letter topic + policy)" : "(sibling DLQ + redrive)"}</span>
          </label>
          <DeployIntoField provider={provider} providerType={selectedType} value={targetAccount} onChange={setTargetAccount} />
          <div className="flex items-start gap-3 rounded-lg border border-border bg-muted/40 p-3 sm:col-span-2">
            <Inbox className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
            <div className="text-sm text-muted-foreground">
              {isAzure
                ? "A Service Bus namespace is created in its own resource group with one queue. The connection string (SAS key) is never persisted by OPORD."
                : isGCP
                  ? "A Pub/Sub topic and pull subscription are created. Enable dead-letter to capture undeliverable messages."
                  : "An SQS queue is created with server-side encryption. Enable FIFO for ordered, exactly-once delivery, or a DLQ to capture poison messages."}
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <button type="submit" disabled={submitting} className={cn(button({ size: "md" }), submitting && "opacity-70")}>
          {submitting && <Loader2 className="size-4 animate-spin" />}
          {isAzure ? "Create Service Bus queue" : isGCP ? "Create Pub/Sub topic" : "Create SQS queue"}
        </button>
        <Link href="/queues" className={button({ variant: "outline", size: "md" })}>
          Cancel
        </Link>
      </div>
    </form>
  );
}
