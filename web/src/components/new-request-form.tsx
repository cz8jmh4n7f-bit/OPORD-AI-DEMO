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

const API = "/bff";

const inputCls =
  "h-9 w-full rounded-lg border border-input bg-card px-3 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";

const kinds = ["vm", "cluster", "database", "stack", "environment"];

const specHints: Record<string, string> = {
  vm: '{\n  "template": "9000",\n  "count": 1,\n  "cpu": 2,\n  "memory_mb": 4096,\n  "disk_gb": 40,\n  "ip_start": "10.0.0.10",\n  "gateway": "10.0.0.1",\n  "ssh_user": "debian"\n}',
  database: '{\n  "engine": "postgres",\n  "version": "16",\n  "instance_class": "db.t3.micro",\n  "storage_gb": 20,\n  "db_name": "app",\n  "username": "appuser"\n}',
  stack: '{\n  "module_dir": "modules/examples/s3-bucket",\n  "variables": { "region": "eu-central-1", "bucket_name": "opord-demo" }\n}',
  cluster: "{}",
  environment: "{}",
};

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-xs font-medium text-muted-foreground">{label}</span>
      {children}
      {hint && <span className="text-[11px] text-muted-foreground">{hint}</span>}
    </label>
  );
}

export function NewRequestForm({
  providers,
  initialProvider,
  initialKind,
}: {
  providers: Provider[];
  initialProvider?: string;
  initialKind?: string;
}) {
  const router = useRouter();

  const presetProvider =
    initialProvider && providers.some((p) => p.name === initialProvider) ? initialProvider : undefined;
  const presetKind = initialKind && kinds.includes(initialKind) ? initialKind : "stack";

  const [name, setName] = useState("");
  const [environment, setEnvironment] = useState("dev");
  const [requester, setRequester] = useState("");
  const [kind, setKind] = useState(presetKind);
  const [provider, setProvider] = useState(presetProvider ?? providers[0]?.name ?? "");
  const [blueprint, setBlueprint] = useState("");
  const [specText, setSpecText] = useState(specHints[presetKind] ?? "{}");

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);

  function onKindChange(k: string) {
    setKind(k);
    setSpecText(specHints[k] ?? "{}");
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setOk(null);

    let spec: unknown = {};
    if (specText.trim()) {
      try {
        spec = JSON.parse(specText);
      } catch {
        setError("Spec must be valid JSON.");
        return;
      }
    }

    setSubmitting(true);
    try {
      const res = await fetch(`${API}/api/v1/requests`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ name, environment, requester, kind, provider, blueprint, spec }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `request failed (${res.status})`);
        return;
      }
      setOk(`Request "${data.name}" submitted - ${data.status}.`);
      router.refresh();
      setTimeout(() => router.push("/requests"), 900);
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
          <CardTitle>Request</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Name">
            <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} placeholder="my-resource" required />
          </Field>
          <Field label="Environment">
            <select className={inputCls} value={environment} onChange={(e) => setEnvironment(e.target.value)}>
              <option value="dev">dev</option>
              <option value="staging">staging</option>
              <option value="production">production</option>
            </select>
          </Field>
          <Field label="Requester">
            <input className={inputCls} value={requester} onChange={(e) => setRequester(e.target.value)} placeholder="you@example.com" />
          </Field>
          <Field label="Kind">
            <select className={inputCls} value={kind} onChange={(e) => onKindChange(e.target.value)}>
              {kinds.map((k) => (
                <option key={k} value={k}>{k}</option>
              ))}
            </select>
          </Field>
          <Field label="Provider">
            <select className={inputCls} value={provider} onChange={(e) => setProvider(e.target.value)} required>
              {providers.length === 0 && <option value="">no providers</option>}
              {providers.map((p) => (
                <option key={p.id} value={p.name}>{p.name} ({p.type})</option>
              ))}
            </select>
          </Field>
          <Field label="Blueprint" hint="For kind=environment only.">
            <input className={inputCls} value={blueprint} onChange={(e) => setBlueprint(e.target.value)} placeholder="aws-web-stack" />
          </Field>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Spec (JSON)</CardTitle>
        </CardHeader>
        <CardContent>
          <textarea
            className="h-56 w-full rounded-lg border border-border bg-card p-3 font-mono text-xs text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            value={specText}
            onChange={(e) => setSpecText(e.target.value)}
            spellCheck={false}
          />
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <button type="submit" disabled={submitting} className={cn(button({ size: "md" }), submitting && "opacity-70")}>
          {submitting && <Loader2 className="size-4 animate-spin" />}
          Submit request
        </button>
        <Link href="/requests" className={button({ variant: "outline", size: "md" })}>
          Cancel
        </Link>
      </div>
    </form>
  );
}
