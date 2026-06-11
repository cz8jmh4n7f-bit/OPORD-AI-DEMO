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

export function NewTableForm({ providers, initialProvider }: { providers: Provider[]; initialProvider?: string }) {
  const router = useRouter();
  const preset = initialProvider && providers.some((p) => p.name === initialProvider) ? initialProvider : undefined;

  const [name, setName] = useState("");
  const [environment, setEnvironment] = useState("dev");
  const [provider, setProvider] = useState(preset ?? providers.find((p) => p.type === "aws")?.name ?? providers[0]?.name ?? "");
  const [hashKey, setHashKey] = useState("id");
  const [hashKeyType, setHashKeyType] = useState("S");
  const [rangeKey, setRangeKey] = useState("");
  const [rangeKeyType, setRangeKeyType] = useState("S");
  const [billingMode, setBillingMode] = useState("PAY_PER_REQUEST");
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
        hash_key: hashKey,
        hash_key_type: hashKeyType,
        billing_mode: billingMode,
      };
      if (rangeKey.trim()) {
        spec.range_key = rangeKey.trim();
        spec.range_key_type = rangeKeyType;
      }
      // Deploy into a OPORD-managed account (ADR-0013) instead of the provider's
      // default - GCP project / Azure subscription / AWS member account (cross-account AssumeRole).
      if ((isAws || isGCP || isAzure) && targetAccount.trim()) spec.target_account = targetAccount.trim();
      const res = await fetch(`${API}/api/v1/tables`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ name, environment, provider, spec }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `request failed (${res.status})`);
        return;
      }
      setOk(`Table "${data.name}" created - status ${data.status}.`);
      router.refresh();
      setTimeout(() => router.push("/tables"), 900);
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
          <CardTitle>{isAzure ? "Table (Cosmos DB)" : isGCP ? "Table (Firestore)" : "Table (DynamoDB)"}</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Name">
            <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} placeholder="sessions" required />
          </Field>
          <Field label="Environment">
            <select className={inputCls} value={environment} onChange={(e) => setEnvironment(e.target.value)}>
              <option value="dev">dev</option>
              <option value="staging">staging</option>
              <option value="production">production</option>
            </select>
          </Field>
          <Field label="Provider" hint="AWS (DynamoDB), Azure (Cosmos DB), or GCP (Firestore).">
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
          <Field label="Billing mode" hint={isGCP || isAzure ? "On-demand pricing; ignored by Firestore and Cosmos DB." : undefined}>
            <select className={inputCls} value={billingMode} onChange={(e) => setBillingMode(e.target.value)}>
              <option value="PAY_PER_REQUEST">PAY_PER_REQUEST (on-demand)</option>
              <option value="PROVISIONED">PROVISIONED</option>
            </select>
          </Field>
          <Field label={isGCP || isAzure ? "Partition key" : "Hash key"} hint={isGCP ? "The Firestore collection ID / document field used as the partition key." : isAzure ? "The Cosmos DB partition key path (e.g. /id)." : "DynamoDB partition key attribute name."}>
            <input className={inputCls} value={hashKey} onChange={(e) => setHashKey(e.target.value)} placeholder="id" required />
          </Field>
          <Field label={isGCP || isAzure ? "Partition key type" : "Hash key type"}>
            <select className={inputCls} value={hashKeyType} onChange={(e) => setHashKeyType(e.target.value)}>
              <option value="S">String (S)</option>
              <option value="N">Number (N)</option>
              <option value="B">Binary (B)</option>
            </select>
          </Field>
          <Field label={isGCP || isAzure ? "Sort key" : "Range key"} hint={isGCP || isAzure ? "Optional secondary sort attribute." : "Optional sort key."}>
            <input className={inputCls} value={rangeKey} onChange={(e) => setRangeKey(e.target.value)} placeholder="(none)" />
          </Field>
          <Field label={isGCP || isAzure ? "Sort key type" : "Range key type"}>
            <select className={inputCls} value={rangeKeyType} onChange={(e) => setRangeKeyType(e.target.value)} disabled={rangeKey.trim() === ""}>
              <option value="S">String (S)</option>
              <option value="N">Number (N)</option>
              <option value="B">Binary (B)</option>
            </select>
          </Field>
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <button type="submit" disabled={submitting} className={cn(button({ size: "md" }), submitting && "opacity-70")}>
          {submitting && <Loader2 className="size-4 animate-spin" />}
          {isAzure ? "Create Cosmos DB table" : isGCP ? "Create Firestore table" : "Create DynamoDB table"}
        </button>
        <Link href="/tables" className={button({ variant: "outline", size: "md" })}>
          Cancel
        </Link>
      </div>
    </form>
  );
}
