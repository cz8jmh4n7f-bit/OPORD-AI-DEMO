"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Check, Loader2, TriangleAlert } from "lucide-react";
import type { Blueprint, Provider } from "@/lib/types";
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

export function NewEnvironmentForm({
  providers,
  blueprints,
  initialProvider,
}: {
  providers: Provider[];
  blueprints: Blueprint[];
  initialProvider?: string;
}) {
  const router = useRouter();

  const presetProvider =
    initialProvider && providers.some((p) => p.name === initialProvider) ? initialProvider : undefined;

  const [name, setName] = useState("");
  const [environment, setEnvironment] = useState("dev");
  const [provider, setProvider] = useState(presetProvider ?? providers[0]?.name ?? "");
  const [blueprint, setBlueprint] = useState(blueprints[0]?.id ?? "");
  const [sshKey, setSshKey] = useState("");
  const [template, setTemplate] = useState("");

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);

  const selected = blueprints.find((b) => b.id === blueprint);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    setOk(null);
    try {
      const res = await fetch(`${API}/api/v1/environments`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ name, environment, provider, blueprint, sshPublicKey: sshKey, template }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `request failed (${res.status})`);
        return;
      }
      setOk(`Environment "${data.name}" created - status ${data.status}.`);
      router.refresh();
      setTimeout(() => router.push("/environments"), 900);
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
          <CardTitle>Basics</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Name">
            <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} placeholder="staging" required />
          </Field>
          <Field label="Environment">
            <select className={inputCls} value={environment} onChange={(e) => setEnvironment(e.target.value)}>
              <option value="dev">dev</option>
              <option value="staging">staging</option>
              <option value="production">production</option>
            </select>
          </Field>
          <Field label="Provider" hint="Backend the whole environment lands on.">
            <select className={inputCls} value={provider} onChange={(e) => setProvider(e.target.value)} required>
              {providers.length === 0 && <option value="">no providers registered</option>}
              {providers.map((p) => (
                <option key={p.id} value={p.name}>
                  {p.name} ({p.type})
                </option>
              ))}
            </select>
          </Field>
          <Field label="Blueprint" hint="Golden path to instantiate.">
            <select className={inputCls} value={blueprint} onChange={(e) => setBlueprint(e.target.value)} required>
              {blueprints.length === 0 && <option value="">no blueprints</option>}
              {blueprints.map((b) => (
                <option key={b.id} value={b.id}>
                  {b.name}
                </option>
              ))}
            </select>
          </Field>
        </CardContent>
      </Card>

      {selected && (
        <Card>
          <CardHeader>
            <CardTitle>{selected.name}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <p className="text-sm text-muted-foreground">{selected.description}</p>
            <div className="flex flex-wrap gap-2">
              {selected.components.map((c) => (
                <span key={c.name} className="rounded-md border border-border bg-muted px-2 py-1 text-xs">
                  <span className="font-medium">{c.name}</span>{" "}
                  <span className="text-muted-foreground">· {c.kind}</span>
                </span>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Options</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Template / image" hint="Override the golden image / VMID / AMI (optional).">
            <input className={inputCls} value={template} onChange={(e) => setTemplate(e.target.value)} placeholder="9000" />
          </Field>
          <Field label="SSH public key" hint="Injected into the nodes.">
            <input className={inputCls} value={sshKey} onChange={(e) => setSshKey(e.target.value)} placeholder="ssh-rsa AAAA..." />
          </Field>
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <button type="submit" disabled={submitting} className={cn(button({ size: "md" }), submitting && "opacity-70")}>
          {submitting && <Loader2 className="size-4 animate-spin" />}
          Create environment
        </button>
        <Link href="/environments" className={button({ variant: "outline", size: "md" })}>
          Cancel
        </Link>
      </div>
    </form>
  );
}
