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

export function NewDatabaseForm({ providers, initialProvider }: { providers: Provider[]; initialProvider?: string }) {
  const router = useRouter();
  const preset = initialProvider && providers.some((p) => p.name === initialProvider) ? initialProvider : undefined;

  const [name, setName] = useState("");
  const [environment, setEnvironment] = useState("dev");
  const [provider, setProvider] = useState(preset ?? providers.find((p) => p.type === "aws")?.name ?? providers[0]?.name ?? "");
  const [engine, setEngine] = useState("postgres");
  const [version, setVersion] = useState("16");
  const [instanceClass, setInstanceClass] = useState("db.t3.micro");
  const [storageGb, setStorageGb] = useState(20);
  const [dbName, setDbName] = useState("app");
  const [username, setUsername] = useState("appuser");

  const [targetAccount, setTargetAccount] = useState("");
  const [authMode, setAuthMode] = useState("password");
  const [authPrincipal, setAuthPrincipal] = useState("");

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);

  // Cloud provider type for the selected provider - drives the "Deploy into" field.
  const selectedType = providers.find((p) => p.name === provider)?.type;
  const isAws = selectedType === "aws";
  const isGCP = selectedType === "gcp";
  const isAzure = selectedType === "azure";

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setOk(null);
    setSubmitting(true);
    try {
      const spec: Record<string, unknown> = {
        engine,
        version,
        instance_class: instanceClass,
        storage_gb: storageGb,
        db_name: dbName,
        username,
      };
      // Deploy into a OPORD-managed account (ADR-0013) instead of the provider's
      // default - GCP project / Azure subscription / AWS member account (cross-account AssumeRole).
      if ((isAws || isGCP || isAzure) && targetAccount.trim()) spec.target_account = targetAccount.trim();
      // Passwordless IAM database auth (GCP Cloud SQL) - no static password; the
      // principal connects with a short-lived IAM token.
      if (isGCP && authMode === "iam") {
        spec.auth_mode = "iam";
        spec.auth_principal = authPrincipal.trim();
      }

      const res = await fetch(`${API}/api/v1/databases`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ name, environment, provider, spec }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `request failed (${res.status})`);
        return;
      }
      setOk(`Database "${data.name}" created - status ${data.status}.`);
      router.refresh();
      setTimeout(() => router.push("/databases"), 900);
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
          <CardTitle>{isAzure ? "Database (PostgreSQL Flexible Server)" : isGCP ? "Database (Cloud SQL)" : "Database (RDS)"}</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Name">
            <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} placeholder="app-db" required />
          </Field>
          <Field label="Environment">
            <select className={inputCls} value={environment} onChange={(e) => setEnvironment(e.target.value)}>
              <option value="dev">dev</option>
              <option value="staging">staging</option>
              <option value="production">production</option>
            </select>
          </Field>
          <Field label="Provider" hint={isAzure ? "Azure Database for PostgreSQL (Flexible Server)." : isGCP ? "GCP Cloud SQL." : "AWS RDS. Needs kms perms for the managed master password."}>
            <select className={inputCls} value={provider} onChange={(e) => { setProvider(e.target.value); setTargetAccount(""); }} required>
              {providers.length === 0 && <option value="">no providers registered</option>}
              {providers.map((p) => (
                <option key={p.id} value={p.name}>
                  {p.name} ({p.type})
                </option>
              ))}
            </select>
          </Field>
          <DeployIntoField provider={provider} providerType={selectedType} value={targetAccount} onChange={setTargetAccount} />
          <Field label="Engine">
            <select
              className={inputCls}
              value={engine}
              onChange={(e) => {
                const ng = e.target.value;
                setEngine(ng);
                // Reset to a valid major for the new engine - MySQL has no v16
                // (that's Postgres), so the cross-engine default would build an
                // invalid Cloud SQL database_version like MYSQL_16.
                setVersion(ng === "mysql" ? "8.0" : "16");
              }}
            >
              <option value="postgres">postgres</option>
              <option value="mysql">mysql</option>
            </select>
          </Field>
          <Field label="Version" hint={engine === "mysql" ? "MySQL major version." : "PostgreSQL major version."}>
            <select className={inputCls} value={version} onChange={(e) => setVersion(e.target.value)}>
              {(engine === "mysql" ? ["8.0", "8.4", "5.7"] : ["16", "15", "14", "13"]).map((v) => (
                <option key={v} value={v}>
                  {v}
                </option>
              ))}
            </select>
          </Field>
          <Field label="Instance class">
            <input className={inputCls} value={instanceClass} onChange={(e) => setInstanceClass(e.target.value)} placeholder="db.t3.micro" />
          </Field>
          <Field label="Storage (GB)">
            <input type="number" min={20} className={inputCls} value={storageGb} onChange={(e) => setStorageGb(Number(e.target.value))} />
          </Field>
          <Field label="DB name">
            <input className={inputCls} value={dbName} onChange={(e) => setDbName(e.target.value)} placeholder="app" />
          </Field>
          {isGCP && (
            <Field label="Authentication" hint="IAM = passwordless: the principal connects with a short-lived IAM token; OPORD stores no password.">
              <select className={inputCls} value={authMode} onChange={(e) => setAuthMode(e.target.value)}>
                <option value="password">Password (stored in OpenBao)</option>
                <option value="iam">IAM (passwordless)</option>
              </select>
            </Field>
          )}
          {isGCP && authMode === "iam" ? (
            <Field label="IAM principal" hint="IAM user email or service-account email granted DB access.">
              <input className={inputCls} value={authPrincipal} onChange={(e) => setAuthPrincipal(e.target.value)} placeholder="dev@example.com" required />
            </Field>
          ) : (
            <Field label="Master username" hint={isAws ? "RDS manages the password (OPORD never holds it)." : "OPORD stores the generated password in OpenBao (opord/databases/<name>); it's also in tofu state."}>
              <input className={inputCls} value={username} onChange={(e) => setUsername(e.target.value)} placeholder="appuser" />
            </Field>
          )}
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <button type="submit" disabled={submitting} className={cn(button({ size: "md" }), submitting && "opacity-70")}>
          {submitting && <Loader2 className="size-4 animate-spin" />}
          Create database
        </button>
        <Link href="/databases" className={button({ variant: "outline", size: "md" })}>
          Cancel
        </Link>
      </div>
    </form>
  );
}
