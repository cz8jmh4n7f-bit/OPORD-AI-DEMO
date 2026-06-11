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

export function NewStackForm({ providers, initialProvider }: { providers: Provider[]; initialProvider?: string }) {
  const router = useRouter();

  const presetProvider =
    initialProvider && providers.some((p) => p.name === initialProvider) ? initialProvider : undefined;

  const [name, setName] = useState("");
  const [environment, setEnvironment] = useState("dev");
  const [provider, setProvider] = useState(
    presetProvider ?? providers.find((p) => p.type === "aws")?.name ?? providers[0]?.name ?? "",
  );
  const [moduleDir, setModuleDir] = useState("modules/examples/s3-bucket");
  const [variablesText, setVariablesText] = useState(
    '{\n  "region": "eu-central-1",\n  "bucket_name": "opord-demo-1"\n}',
  );

  const [targetAccount, setTargetAccount] = useState("");

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);

  // Cloud providers (AWS/GCP/Azure): determine type for the "Deploy into" dropdown.
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

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setOk(null);

    let variables: Record<string, unknown> = {};
    if (variablesText.trim()) {
      try {
        variables = JSON.parse(variablesText);
      } catch {
        setError("Variables must be valid JSON.");
        return;
      }
    }

    setSubmitting(true);
    try {
      const body: Record<string, unknown> = { name, environment, provider, moduleDir, variables };
      if ((isAws || isGCP || isAzure) && targetAccount.trim()) body.target_account = targetAccount.trim();

      const res = await fetch(`${API}/api/v1/stacks`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify(body),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `request failed (${res.status})`);
        return;
      }
      setOk(`Stack "${data.name}" created - status ${data.status}.`);
      router.refresh();
      setTimeout(() => router.push("/stacks"), 900);
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
          <CardTitle>Stack</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Name">
            <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} placeholder="assets" required />
          </Field>
          <Field label="Environment">
            <select className={inputCls} value={environment} onChange={(e) => setEnvironment(e.target.value)}>
              <option value="dev">dev</option>
              <option value="staging">staging</option>
              <option value="production">production</option>
            </select>
          </Field>
          <Field label="Provider" hint="Any cloud provider that supports generic OpenTofu stacks (AWS, GCP, Azure).">
            <select className={inputCls} value={provider} onChange={(e) => setProvider(e.target.value)} required>
              {providers.length === 0 && <option value="">no providers registered</option>}
              {providers.map((p) => (
                <option key={p.id} value={p.name}>
                  {p.name} ({p.type})
                </option>
              ))}
            </select>
          </Field>
          <Field label="Module directory" hint="Path to an OpenTofu root module (no backend block).">
            <input className={inputCls} value={moduleDir} onChange={(e) => setModuleDir(e.target.value)} placeholder="modules/examples/s3-bucket" required />
          </Field>
          <DeployIntoField provider={provider} providerType={selectedType} value={targetAccount} onChange={setTargetAccount} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Variables (JSON)</CardTitle>
        </CardHeader>
        <CardContent>
          <textarea
            className="h-48 w-full rounded-lg border border-border bg-card p-3 font-mono text-xs text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            value={variablesText}
            onChange={(e) => setVariablesText(e.target.value)}
            spellCheck={false}
          />
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <button type="submit" disabled={submitting} className={cn(button({ size: "md" }), submitting && "opacity-70")}>
          {submitting && <Loader2 className="size-4 animate-spin" />}
          Create stack
        </button>
        <Link href="/stacks" className={button({ variant: "outline", size: "md" })}>
          Cancel
        </Link>
      </div>
    </form>
  );
}
