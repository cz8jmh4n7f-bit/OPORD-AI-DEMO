"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { createPortal } from "react-dom";
import { Loader2, Plus, UploadCloud, WandSparkles } from "lucide-react";
import { authHeaders } from "@/lib/client-auth";
import { cn } from "@/lib/utils";
import { button } from "@/components/ui/button";
import { useToast } from "@/components/ui/toast";

const API = "/bff";
const inputCls =
  "h-9 w-full rounded-lg border border-input bg-card px-3 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";
const textCls =
  "min-h-24 w-full rounded-lg border border-input bg-card px-3 py-2 font-mono text-xs text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";

type ModalKind = "budget" | "quota" | "policy";

export function AddAIGovernanceButton({ kind }: { kind: ModalKind }) {
  const router = useRouter();
  const { toast } = useToast();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [name, setName] = useState(kind === "policy" ? "default-ai-access-policy" : "");
  const [scope, setScope] = useState("global");
  const [scopeRef, setScopeRef] = useState("");
  const [limit, setLimit] = useState(kind === "budget" ? "100" : "1000000");
  const [period, setPeriod] = useState("monthly");
  const [serviceSlug, setServiceSlug] = useState("");
  const [metric, setMetric] = useState("tokens");
  const [enforcement, setEnforcement] = useState("warn");
  const [rules, setRules] = useState('{"allowed_models":[],"requires_justification":true,"max_expiration_days":90}');

  const labels = {
    budget: "Add AI budget",
    quota: "Add AI quota",
    policy: "Add AI policy",
  };

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      let path = "";
      let body: Record<string, unknown> = {};
      if (kind === "budget") {
        path = "/api/v1/ai/budgets";
        body = { scope, scopeRef, limitUsd: Number(limit), period, softThresholdPct: 80, hardThresholdPct: 100 };
      } else if (kind === "quota") {
        path = "/api/v1/ai/quotas";
        body = { serviceSlug, metric, limitQuantity: Number(limit), period, enforcement };
      } else {
        path = "/api/v1/ai/policies";
        body = { name, rules: JSON.parse(rules), status: "active" };
      }
      const res = await fetch(`${API}${path}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify(body),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        toast({ variant: "error", title: "Create failed", description: data.error ?? `Request failed (${res.status})` });
        return;
      }
      toast({ variant: "success", title: `${labels[kind]} created` });
      setOpen(false);
      router.refresh();
    } catch (err) {
      toast({ variant: "error", title: "Create failed", description: String(err) });
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <button type="button" onClick={() => setOpen(true)} className={button({ variant: "outline", size: "md" })}>
        <Plus className="size-4" />
        {labels[kind]}
      </button>
      {open &&
        typeof document !== "undefined" &&
        createPortal(
          <div className="fixed inset-0 z-[70] flex items-center justify-center p-4" role="dialog" aria-modal="true">
            <div className="absolute inset-0 bg-black/50" onClick={() => !busy && setOpen(false)} />
            <form onSubmit={submit} className="relative w-full max-w-lg space-y-4 rounded-xl border border-border bg-card p-5 shadow-xl">
              <div>
                <h2 className="text-base font-semibold text-foreground">{labels[kind]}</h2>
                <p className="mt-1 text-xs text-muted-foreground">Governance metadata only; enforcement can be tightened as gateway traffic grows.</p>
              </div>
              {kind === "budget" && (
                <>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Scope</span>
                    <select className={inputCls} value={scope} onChange={(e) => setScope(e.target.value)}>
                      <option value="global">Global</option>
                      <option value="provider">Provider</option>
                      <option value="owner">Owner</option>
                      <option value="workspace">Workspace</option>
                    </select>
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Scope reference</span>
                    <input className={inputCls} value={scopeRef} onChange={(e) => setScopeRef(e.target.value)} placeholder="openai-main, team-a, workspace-a" />
                  </label>
                </>
              )}
              {kind === "quota" && (
                <>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Service slug</span>
                    <input className={inputCls} value={serviceSlug} onChange={(e) => setServiceSlug(e.target.value)} placeholder="optional, e.g. openai-api-access" />
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Metric</span>
                    <input className={inputCls} value={metric} onChange={(e) => setMetric(e.target.value)} />
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Enforcement</span>
                    <select className={inputCls} value={enforcement} onChange={(e) => setEnforcement(e.target.value)}>
                      <option value="warn">Warn</option>
                      <option value="block">Block</option>
                    </select>
                  </label>
                </>
              )}
              {kind === "policy" && (
                <>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Name</span>
                    <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} required />
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Rules JSON</span>
                    <textarea className={textCls} value={rules} onChange={(e) => setRules(e.target.value)} />
                  </label>
                </>
              )}
              {kind !== "policy" && (
                <div className="grid gap-3 sm:grid-cols-2">
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">{kind === "budget" ? "Limit USD" : "Limit quantity"}</span>
                    <input className={inputCls} value={limit} onChange={(e) => setLimit(e.target.value)} inputMode="decimal" required />
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Period</span>
                    <select className={inputCls} value={period} onChange={(e) => setPeriod(e.target.value)}>
                      <option value="daily">Daily</option>
                      <option value="monthly">Monthly</option>
                      <option value="yearly">Yearly</option>
                    </select>
                  </label>
                </div>
              )}
              <div className="flex items-center justify-end gap-2">
                <button type="button" onClick={() => setOpen(false)} disabled={busy} className={button({ variant: "outline", size: "sm" })}>
                  Cancel
                </button>
                <button type="submit" disabled={busy} className={cn(button({ size: "sm" }), busy && "opacity-70")}>
                  {busy && <Loader2 className="size-4 animate-spin" />}
                  Create
                </button>
              </div>
            </form>
          </div>,
          document.body,
        )}
    </>
  );
}

export function ImportOpenAIUsageButton() {
  const router = useRouter();
  const { toast } = useToast();
  const [busy, setBusy] = useState(false);

  async function run() {
    setBusy(true);
    try {
      const res = await fetch(`${API}/api/v1/ai/usage/import/openai`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ providerName: "openai-main" }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        toast({ variant: "error", title: "Import failed", description: data.error ?? `Request failed (${res.status})` });
        return;
      }
      toast({ variant: "success", title: "OpenAI costs imported", description: `${data.imported ?? 0} imported, ${data.skipped ?? 0} skipped.` });
      router.refresh();
    } catch (err) {
      toast({ variant: "error", title: "Import failed", description: String(err) });
    } finally {
      setBusy(false);
    }
  }

  return (
    <button type="button" onClick={run} disabled={busy} className={cn(button({ variant: "outline", size: "md" }), busy && "opacity-70")}>
      {busy ? <Loader2 className="size-4 animate-spin" /> : <UploadCloud className="size-4" />}
      Import OpenAI costs
    </button>
  );
}

export function GatewaySmokeButton() {
  const { toast } = useToast();
  const [busy, setBusy] = useState(false);

  async function run() {
    setBusy(true);
    try {
      const res = await fetch(`${API}/api/v1/ai/gateway/openai/responses?provider=openai-main`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({
          model: "gpt-5-mini",
          input: "Reply with one short sentence confirming the OPORD AI gateway works.",
        }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        toast({ variant: "error", title: "Gateway failed", description: data.error ?? `Request failed (${res.status})` });
        return;
      }
      toast({ variant: "success", title: "Gateway response received", description: data.output_text ?? data.id ?? "OpenAI response proxied through OPORD." });
    } catch (err) {
      toast({ variant: "error", title: "Gateway failed", description: String(err) });
    } finally {
      setBusy(false);
    }
  }

  return (
    <button type="button" onClick={run} disabled={busy} className={cn(button({ variant: "outline", size: "md" }), busy && "opacity-70")}>
      {busy ? <Loader2 className="size-4 animate-spin" /> : <WandSparkles className="size-4" />}
      Smoke test gateway
    </button>
  );
}
