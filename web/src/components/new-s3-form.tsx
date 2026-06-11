"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Archive, Check, Loader2, TriangleAlert } from "lucide-react";
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

export function NewS3Form({ providers, initialProvider }: { providers: Provider[]; initialProvider?: string }) {
  const router = useRouter();
  // Object storage works on AWS (S3), Azure (Storage Account), and GCP (GCS) -
  // all implement the S3Provisioner capability.
  const storageProviders = providers.filter((p) => p.type === "aws" || p.type === "azure" || p.type === "gcp");
  const preset = initialProvider && providers.some((p) => p.name === initialProvider) ? initialProvider : undefined;

  const [name, setName] = useState("");
  const [bucketName, setBucketName] = useState("");
  const [environment, setEnvironment] = useState("dev");
  const [provider, setProvider] = useState(preset ?? storageProviders[0]?.name ?? providers[0]?.name ?? "");
  const [kmsKeyArn, setKmsKeyArn] = useState("");
  const [lifecycleGlacierDays, setLifecycleGlacierDays] = useState("0");

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
        name: bucketName.trim() || name.trim(),
        versioning: true,
        block_public_access: true,
      };
      // KMS + Glacier lifecycle are AWS-only; Azure Storage ignores them.
      if (!isAzure && !isGCP) {
        if (kmsKeyArn.trim()) spec.kms_key_arn = kmsKeyArn.trim();
        const glacierDays = Number(lifecycleGlacierDays);
        if (Number.isFinite(glacierDays) && glacierDays > 0) spec.lifecycle_glacier_days = glacierDays;
      }
      // Deploy into a OPORD-managed account (ADR-0013) - GCP project / Azure subscription /
      // AWS member account (cross-account AssumeRole).
      if ((isAws || isGCP || isAzure) && targetAccount.trim()) spec.target_account = targetAccount.trim();

      const res = await fetch(`${API}/api/v1/s3`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ name, environment, provider, spec }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `request failed (${res.status})`);
        return;
      }
      setOk(`S3 bucket "${data.name}" created - status ${data.status}.`);
      router.refresh();
      setTimeout(() => router.push("/s3"), 900);
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
          <CardTitle>{isAzure ? "Object storage (Azure Storage Account)" : isGCP ? "Object storage (Google Cloud Storage)" : "Bucket (S3)"}</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Resource name" hint="OPORD tracking name.">
            <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} placeholder="assets" required />
          </Field>
          <Field label={isAzure ? "Storage name prefix" : "Bucket name"} hint="Optional. Defaults to the resource name.">
            <input className={inputCls} value={bucketName} onChange={(e) => setBucketName(e.target.value)} placeholder={isAzure ? "opordassets" : "opord-assets-dev"} />
          </Field>
          <Field label="Environment">
            <select className={inputCls} value={environment} onChange={(e) => setEnvironment(e.target.value)}>
              <option value="dev">dev</option>
              <option value="staging">staging</option>
              <option value="production">production</option>
            </select>
          </Field>
          <Field label="Provider" hint="AWS (S3), Azure (Storage Account), or GCP (Cloud Storage).">
            <select className={inputCls} value={provider} onChange={(e) => setProvider(e.target.value)} required>
              {providers.length === 0 && <option value="">no providers registered</option>}
              {(storageProviders.length > 0 ? storageProviders : providers).map((p) => (
                <option key={p.id} value={p.name}>
                  {p.name} ({p.type})
                </option>
              ))}
            </select>
          </Field>
          <DeployIntoField provider={provider} providerType={selectedType} value={targetAccount} onChange={setTargetAccount} />
          {!isAzure && !isGCP && (
            <>
              <Field label="KMS key ARN" hint="Optional customer-managed key.">
                <input className={inputCls} value={kmsKeyArn} onChange={(e) => setKmsKeyArn(e.target.value)} placeholder="arn:aws:kms:..." />
              </Field>
              <Field label="Glacier after days" hint="0 keeps the lifecycle rule disabled.">
                <input
                  className={inputCls}
                  type="number"
                  min="0"
                  value={lifecycleGlacierDays}
                  onChange={(e) => setLifecycleGlacierDays(e.target.value)}
                />
              </Field>
            </>
          )}
          <div className="flex items-start gap-3 rounded-lg border border-border bg-muted/40 p-3 sm:col-span-2">
            <Archive className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
            <div className="text-sm text-muted-foreground">
              {isAzure
                ? "A Storage Account is created in its own resource group with versioning on, public blob access off, TLS 1.2 min, and a default 'data' container."
                : isGCP
                  ? "A Cloud Storage bucket is created with uniform access control, public access prevention enforced, and versioning enabled."
                  : "Buckets are created private with versioning and public access block enabled. Destruction uses OPORD teardown so objects and Terraform state stay under one lifecycle."}
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <button type="submit" disabled={submitting} className={cn(button({ size: "md" }), submitting && "opacity-70")}>
          {submitting && <Loader2 className="size-4 animate-spin" />}
          {isAzure ? "Create storage account" : isGCP ? "Create GCS bucket" : "Create bucket"}
        </button>
        <Link href="/s3" className={button({ variant: "outline", size: "md" })}>
          Cancel
        </Link>
      </div>
    </form>
  );
}
