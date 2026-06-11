"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { createPortal } from "react-dom";
import { Activity, AlertTriangle, CheckCircle2, ListChecks, Loader2, Pencil, Trash2, XCircle } from "lucide-react";
import type { Provider, ProviderReadiness, ProviderReadinessCheck, ProviderType } from "@/lib/types";
import { button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { authHeaders } from "@/lib/client-auth";
import { useToast } from "@/components/ui/toast";
import { useConfirm } from "@/components/ui/confirm";

const API = "/bff";

const inputCls =
  "h-9 w-full rounded-lg border border-input bg-card px-3 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";

function record(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
}

function str(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function bool(value: unknown): boolean {
  return typeof value === "boolean" ? value : false;
}

function withoutKeys(source: Record<string, unknown>, keys: string[]): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(source)) {
    if (!keys.includes(key)) out[key] = value;
  }
  return out;
}

function ReadinessIcon({ check }: { check: ProviderReadinessCheck }) {
  if (check.status === "ok") return <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-emerald-500" />;
  if (check.status === "failed") return <XCircle className="mt-0.5 size-4 shrink-0 text-danger" />;
  return <AlertTriangle className="mt-0.5 size-4 shrink-0 text-amber-500" />;
}

// ProviderActions adds Edit (config + secret-ref) and Delete to a provider row.
// Edit PATCHes only the connection fields + Vault secret-ref; Delete is guarded
// server-side (refused if clusters/resources still reference the provider).
export function ProviderActions({ provider }: { provider: Provider }) {
  const router = useRouter();
  const { toast } = useToast();
  const { prompt } = useConfirm();

  const [editing, setEditing] = useState(false);
  const [busy, setBusy] = useState(false);
  const [checking, setChecking] = useState(false);
  const [readinessOpen, setReadinessOpen] = useState(false);
  const [readinessBusy, setReadinessBusy] = useState(false);
  const [readiness, setReadiness] = useState<ProviderReadiness | null>(null);

  // check runs a live connectivity probe (POST .../check). The result is also
  // persisted server-side, so we refresh to update the health badge in the row.
  async function check() {
    setChecking(true);
    try {
      const res = await fetch(`${API}/api/v1/providers/${encodeURIComponent(provider.name)}/check`, {
        method: "POST",
        headers: authHeaders(),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        toast({ variant: "error", title: "Check failed", description: data.error ?? `Request failed (${res.status})` });
        return;
      }
      if (data.status === "ok") {
        toast({ variant: "success", title: `“${provider.name}” reachable`, description: `Credentials valid · ${data.latencyMs}ms` });
      } else if (data.status === "unsupported") {
        toast({ variant: "info", title: `Check not supported`, description: data.message });
      } else {
        toast({ variant: "error", title: `“${provider.name}” unreachable`, description: data.message });
      }
      router.refresh();
    } catch (err) {
      toast({ variant: "error", title: "Check failed", description: String(err) });
    } finally {
      setChecking(false);
    }
  }

  const cfg = provider.config ?? {};
  const finops = record(cfg.finops);
  const advancedInitial = JSON.stringify(withoutKeys(cfg, ["region", "location", "zone", "server", "datacenter", "subscription_id", "project_id", "safety_profile", "finops"]), null, 2);

  const [providerName, setProviderName] = useState(provider.name);
  const [providerType, setProviderType] = useState<ProviderType>(provider.type);
  const [secretRef, setSecretRef] = useState(provider.secretRef ?? "");
  const [safetyProfile, setSafetyProfile] = useState(str(cfg.safety_profile) || "dev");
  const [region, setRegion] = useState(provider.region || str(cfg.region) || str(cfg.location));
  const [server, setServer] = useState(provider.server ?? str(cfg.server));
  const [datacenter, setDatacenter] = useState(provider.datacenter ?? str(cfg.datacenter));
  const [advancedJson, setAdvancedJson] = useState(advancedInitial === "{}" ? "" : advancedInitial);
  const [finopsEnabled, setFinopsEnabled] = useState(bool(finops.enabled));
  const [focusVersion, setFocusVersion] = useState(str(finops.focus_version) || "1.2");
  const [focusExportName, setFocusExportName] = useState(str(finops.export_name) || "FOCUS");
  const [focusBucket, setFocusBucket] = useState(str(finops.s3_bucket));
  const [focusPrefix, setFocusPrefix] = useState(str(finops.s3_prefix));
  const [athenaDatabase, setAthenaDatabase] = useState(str(finops.athena_database) || "opord_finops");
  const [athenaTable, setAthenaTable] = useState(str(finops.athena_table) || "focus_aws");
  const [athenaWorkgroup, setAthenaWorkgroup] = useState(str(finops.athena_workgroup) || "primary");
  const [finopsRegion, setFinopsRegion] = useState(str(finops.region) || provider.region || str(cfg.region));
  const [azureSubscriptionId, setAzureSubscriptionId] = useState(str(cfg.subscription_id) || str(finops.subscription_id));
  const [azureExportName, setAzureExportName] = useState(str(finops.export_name) || "opord-focus-cost");
  const [azureFocusVersion, setAzureFocusVersion] = useState(str(finops.focus_version) || "1.2-preview");
  const [azureStorageAccount, setAzureStorageAccount] = useState(str(finops.storage_account));
  const [azureContainer, setAzureContainer] = useState(str(finops.container) || "focus");
  const [azureDirectory, setAzureDirectory] = useState(str(finops.directory) || "azure/focus");
  const [azureFormat, setAzureFormat] = useState(str(finops.format) || "csv");
  const [azureCompression, setAzureCompression] = useState(str(finops.compression) || "gzip");
  const [gcpProjectId, setGcpProjectId] = useState(str(cfg.project_id) || str(finops.project_id));
  const [gcpZone, setGcpZone] = useState(str(cfg.zone));
  const [gcpBillingProjectId, setGcpBillingProjectId] = useState(str(finops.billing_project_id) || str(cfg.project_id));
  const [gcpDataset, setGcpDataset] = useState(str(finops.bigquery_dataset) || "opord_billing");
  const [gcpFocusView, setGcpFocusView] = useState(str(finops.focus_view) || "focus_v1_0");
  const [gcpDetailedTable, setGcpDetailedTable] = useState(str(finops.detailed_export_table));
  const [gcpPricingTable, setGcpPricingTable] = useState(str(finops.pricing_export_table));

  async function showReadiness() {
    setReadinessOpen(true);
    setReadiness(null);
    setReadinessBusy(true);
    try {
      const res = await fetch(`${API}/api/v1/providers/${encodeURIComponent(provider.name)}/readiness`, {
        headers: authHeaders(),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        toast({ variant: "error", title: "Readiness failed", description: data.error ?? `Request failed (${res.status})` });
        return;
      }
      setReadiness(data as ProviderReadiness);
    } catch (err) {
      toast({ variant: "error", title: "Readiness failed", description: String(err) });
    } finally {
      setReadinessBusy(false);
    }
  }

  async function save(e: React.FormEvent) {
    e.preventDefault();
    // PATCH merges only the supplied config keys (UpdateProvider), so the
    // advanced JSON adds/overrides arbitrary keys (subnet_ids, ou_id, …) without
    // clobbering the rest.
    let config: Record<string, unknown> = {};
    if (advancedJson.trim()) {
      try {
        config = { ...JSON.parse(advancedJson) };
      } catch {
        toast({ variant: "error", title: "Advanced config must be valid JSON" });
        return;
      }
    }
    if (providerType === "aws") {
      config.region = region.trim();
      config.safety_profile = safetyProfile;
      if (finopsEnabled || focusBucket.trim() || focusPrefix.trim()) {
        config.finops = {
          enabled: finopsEnabled,
          source: "aws_focus",
          export_name: focusExportName.trim() || "FOCUS",
          focus_version: focusVersion.trim() || "1.2",
          s3_bucket: focusBucket.trim(),
          s3_prefix: focusPrefix.trim().replace(/\/+$/, ""),
          athena_database: athenaDatabase.trim() || "opord_finops",
          athena_table: athenaTable.trim() || "focus_aws",
          athena_workgroup: athenaWorkgroup.trim() || "primary",
          region: finopsRegion.trim() || region.trim(),
        };
      }
    } else if (providerType === "azure") {
      config.location = region.trim();
      config.subscription_id = azureSubscriptionId.trim();
      config.safety_profile = safetyProfile;
      if (finopsEnabled || azureStorageAccount.trim() || azureDirectory.trim()) {
        config.finops = {
          enabled: finopsEnabled,
          source: "azure_focus",
          export_name: azureExportName.trim() || "opord-focus-cost",
          focus_version: azureFocusVersion.trim() || "1.2-preview",
          subscription_id: azureSubscriptionId.trim(),
          storage_account: azureStorageAccount.trim(),
          container: azureContainer.trim() || "focus",
          directory: azureDirectory.trim().replace(/^\/+|\/+$/g, "") || "azure/focus",
          format: azureFormat.trim().toLowerCase() || "csv",
          compression: azureCompression.trim().toLowerCase() || "gzip",
          file_partitioning: true,
          overwrite_data: true,
        };
      }
    } else if (providerType === "gcp") {
      config.project_id = gcpProjectId.trim();
      config.region = region.trim();
      config.zone = gcpZone.trim();
      config.safety_profile = safetyProfile;
      if (finopsEnabled || gcpDataset.trim() || gcpFocusView.trim() || gcpDetailedTable.trim()) {
        config.finops = {
          enabled: finopsEnabled,
          source: "gcp_focus",
          focus_version: "1.0",
          project_id: gcpProjectId.trim(),
          billing_project_id: gcpBillingProjectId.trim() || gcpProjectId.trim(),
          bigquery_dataset: gcpDataset.trim() || "opord_billing",
          focus_view: gcpFocusView.trim() || "focus_v1_0",
          detailed_export_table: gcpDetailedTable.trim(),
          pricing_export_table: gcpPricingTable.trim(),
          region: region.trim(),
        };
      }
    } else {
      config.server = server.trim();
      if (providerType === "vsphere") config.datacenter = datacenter.trim();
    }
    setBusy(true);
    try {
      const res = await fetch(`${API}/api/v1/providers/${encodeURIComponent(provider.name)}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ name: providerName.trim(), type: providerType, secretRef, config }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        toast({ variant: "error", title: "Update failed", description: data.error ?? `Request failed (${res.status})` });
        return;
      }
      toast({ variant: "success", title: `Provider “${provider.name}” updated` });
      setEditing(false);
      router.refresh();
    } catch (err) {
      toast({ variant: "error", title: "Update failed", description: String(err) });
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    const typed = await prompt({
      title: `Delete provider “${provider.name}”?`,
      message:
        "Removes the OPORD registration (refused if clusters or resources still use it). This can't be undone.",
      label: "Type the provider name to confirm",
      placeholder: provider.name,
      requireValue: provider.name,
      confirmLabel: "Delete",
      danger: true,
    });
    if (typed == null) return; // cancelled; requireValue guarantees a match otherwise
    setBusy(true);
    try {
      const res = await fetch(`${API}/api/v1/providers/${encodeURIComponent(provider.name)}`, {
        method: "DELETE",
        headers: authHeaders(),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        toast({ variant: "error", title: "Delete failed", description: data.error ?? `Request failed (${res.status})` });
        return;
      }
      toast({ variant: "success", title: `Provider “${provider.name}” deleted` });
      router.refresh();
    } catch (err) {
      toast({ variant: "error", title: "Delete failed", description: String(err) });
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex items-center justify-end gap-2">
      <button
        type="button"
        onClick={check}
        disabled={checking}
        className={cn(button({ variant: "outline", size: "sm" }))}
        title="Test connection to this provider"
      >
        {checking ? <Loader2 className="size-4 animate-spin" /> : <Activity className="size-4" />}
        Test
      </button>
      <button
        type="button"
        onClick={showReadiness}
        disabled={readinessBusy}
        className={cn(button({ variant: "outline", size: "sm" }))}
        title="Show provisioning and FinOps readiness"
      >
        {readinessBusy ? <Loader2 className="size-4 animate-spin" /> : <ListChecks className="size-4" />}
        Ready
      </button>
      <button
        type="button"
        onClick={() => setEditing(true)}
        className={cn(button({ variant: "outline", size: "sm" }))}
        title="Edit connection / secret reference"
      >
        <Pencil className="size-4" />
        Edit
      </button>
      <button
        type="button"
        onClick={remove}
        disabled={busy}
        className={cn(button({ variant: "danger", size: "sm" }))}
        title="Delete this provider"
      >
        {busy ? <Loader2 className="size-4 animate-spin" /> : <Trash2 className="size-4" />}
        Delete
      </button>

      {readinessOpen &&
        typeof document !== "undefined" &&
        createPortal(
          <div className="fixed inset-0 z-[70] flex items-center justify-center p-4" role="dialog" aria-modal="true">
            <div className="absolute inset-0 bg-black/50" onClick={() => !readinessBusy && setReadinessOpen(false)} />
            <div className="relative max-h-[90vh] w-full max-w-lg space-y-4 overflow-y-auto rounded-xl border border-border bg-card p-5 shadow-xl">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <h2 className="text-base font-semibold text-foreground">Readiness “{provider.name}”</h2>
                  <p className="mt-1 text-xs text-muted-foreground">Provisioning guardrails and FinOps wiring for this provider.</p>
                </div>
                {readiness?.status && (
                  <span
                    className={cn(
                      "rounded-md px-2 py-1 text-xs font-medium",
                      readiness.status === "ok" && "bg-emerald-500/10 text-emerald-600",
                      readiness.status === "warn" && "bg-amber-500/10 text-amber-600",
                      readiness.status === "failed" && "bg-danger/10 text-danger",
                    )}
                  >
                    {readiness.status}
                  </span>
                )}
              </div>
              {readinessBusy && (
                <div className="flex items-center gap-2 rounded-lg border border-border bg-muted/30 p-3 text-sm text-muted-foreground">
                  <Loader2 className="size-4 animate-spin" />
                  Checking provider readiness...
                </div>
              )}
              {!readinessBusy && readiness && (
                <>
                  <div className="space-y-2">
                    {readiness.checks.map((check) => (
                      <div key={check.id} className="flex gap-3 rounded-lg border border-border bg-muted/30 p-3">
                        <ReadinessIcon check={check} />
                        <div className="min-w-0">
                          <p className="text-sm font-medium text-foreground">{check.label}</p>
                          <p className="mt-0.5 text-xs text-muted-foreground">{check.message}</p>
                        </div>
                      </div>
                    ))}
                  </div>
                  <div className="rounded-lg border border-border bg-muted/30 p-3">
                    <p className="text-xs font-medium uppercase text-muted-foreground">Next actions</p>
                    <ul className="mt-2 space-y-1 text-xs text-muted-foreground">
                      {readiness.nextActions.map((action) => (
                        <li key={action}>{action}</li>
                      ))}
                    </ul>
                  </div>
                </>
              )}
              <div className="flex justify-end">
                <button type="button" onClick={() => setReadinessOpen(false)} className={cn(button({ variant: "outline", size: "md" }))}>
                  Close
                </button>
              </div>
            </div>
          </div>,
          document.body,
        )}

      {editing &&
        typeof document !== "undefined" &&
        createPortal(
          <div className="fixed inset-0 z-[70] flex items-center justify-center p-4" role="dialog" aria-modal="true">
            <div className="absolute inset-0 bg-black/50" onClick={() => !busy && setEditing(false)} />
            <form
              onSubmit={save}
              className="relative max-h-[90vh] w-full max-w-md space-y-4 overflow-y-auto rounded-xl border border-border bg-card p-5 shadow-xl"
            >
              <div>
                <h2 className="text-base font-semibold text-foreground">Edit “{provider.name}”</h2>
                <p className="mt-1 text-xs text-muted-foreground">Provider identity, credentials reference, and connection config.</p>
              </div>

              <label className="flex flex-col gap-1.5">
                <span className="text-xs font-medium text-muted-foreground">Provider name</span>
                <input className={inputCls} value={providerName} onChange={(e) => setProviderName(e.target.value)} placeholder="aws-eu" required />
                <span className="text-[11px] text-muted-foreground">Used in catalog forms and resource listings. Existing resources track the provider by ID.</span>
              </label>

              <label className="flex flex-col gap-1.5">
                <span className="text-xs font-medium text-muted-foreground">Provider type</span>
                <select className={inputCls} value={providerType} onChange={(e) => setProviderType(e.target.value as ProviderType)}>
                  <option value="vsphere">vSphere</option>
                  <option value="proxmox">Proxmox VE</option>
                  <option value="aws">AWS</option>
                  <option value="azure">Azure</option>
                  <option value="gcp">Google Cloud</option>
                </select>
                <span className="text-[11px] text-muted-foreground">Type changes are refused if this provider already owns clusters or resources.</span>
              </label>

              {(providerType === "aws" || providerType === "azure" || providerType === "gcp") && (
                <label className="flex flex-col gap-1.5">
                  <span className="text-xs font-medium text-muted-foreground">Safety profile</span>
                  <select className={inputCls} value={safetyProfile} onChange={(e) => setSafetyProfile(e.target.value)}>
                    <option value="sandbox">Sandbox</option>
                    <option value="dev">Dev</option>
                    <option value="prod">Prod</option>
                  </select>
                  <span className="text-[11px] text-muted-foreground">Controls cloud-safe defaults like public access, retention, and destroy-friendly behavior.</span>
                </label>
              )}

              <label className="flex flex-col gap-1.5">
                <span className="text-xs font-medium text-muted-foreground">Secret ref (OpenBao)</span>
                <input
                  className={inputCls}
                  value={secretRef}
                  onChange={(e) => setSecretRef(e.target.value)}
                  placeholder={
                    providerType === "aws"
                      ? "opord/aws/eu"
                      : providerType === "azure"
                        ? "opord/azure/dev"
                        : providerType === "gcp"
                          ? "opord/gcp/dev"
                          : providerType === "proxmox"
                            ? "opord/proxmox/lab"
                            : "opord/vsphere/dev"
                  }
                />
                <span className="text-[11px] text-muted-foreground">
                  KV-v2 path in OpenBao where OPORD reads this provider&apos;s credentials. Empty falls back to the process environment.
                </span>
              </label>
              {providerType === "aws" && (
                <>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Region (AWS)</span>
                    <input className={inputCls} value={region} onChange={(e) => setRegion(e.target.value)} placeholder="eu-central-1" />
                  </label>
                  <section className="space-y-3 rounded-lg border border-border bg-muted/30 p-3">
                    <label className="flex items-center justify-between gap-3">
                      <span>
                        <span className="block text-xs font-medium text-muted-foreground">FinOps FOCUS export</span>
                        <span className="block text-[11px] text-muted-foreground">Connect AWS Data Exports to OPORD actual-cost mode.</span>
                      </span>
                      <input
                        type="checkbox"
                        checked={finopsEnabled}
                        onChange={(e) => setFinopsEnabled(e.target.checked)}
                        className="size-4 accent-primary"
                      />
                    </label>
                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                      <label className="flex flex-col gap-1.5">
                        <span className="text-xs font-medium text-muted-foreground">Export name</span>
                        <input className={inputCls} value={focusExportName} onChange={(e) => setFocusExportName(e.target.value)} placeholder="FOCUS" />
                      </label>
                      <label className="flex flex-col gap-1.5">
                        <span className="text-xs font-medium text-muted-foreground">FOCUS version</span>
                        <input className={inputCls} value={focusVersion} onChange={(e) => setFocusVersion(e.target.value)} placeholder="1.2" />
                      </label>
                    </div>
                    <label className="flex flex-col gap-1.5">
                      <span className="text-xs font-medium text-muted-foreground">S3 bucket</span>
                      <input className={inputCls} value={focusBucket} onChange={(e) => setFocusBucket(e.target.value)} placeholder="opord-finops-focus-123456789012" />
                    </label>
                    <label className="flex flex-col gap-1.5">
                      <span className="text-xs font-medium text-muted-foreground">S3 prefix</span>
                      <input className={inputCls} value={focusPrefix} onChange={(e) => setFocusPrefix(e.target.value)} placeholder="aws/focus" />
                      <span className="text-[11px] text-muted-foreground">No trailing slash; AWS Data Exports rejects prefixes ending in /.</span>
                    </label>
                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                      <label className="flex flex-col gap-1.5">
                        <span className="text-xs font-medium text-muted-foreground">Athena database</span>
                        <input className={inputCls} value={athenaDatabase} onChange={(e) => setAthenaDatabase(e.target.value)} placeholder="opord_finops" />
                      </label>
                      <label className="flex flex-col gap-1.5">
                        <span className="text-xs font-medium text-muted-foreground">Athena table</span>
                        <input className={inputCls} value={athenaTable} onChange={(e) => setAthenaTable(e.target.value)} placeholder="focus_aws" />
                      </label>
                    </div>
                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                      <label className="flex flex-col gap-1.5">
                        <span className="text-xs font-medium text-muted-foreground">Athena workgroup</span>
                        <input className={inputCls} value={athenaWorkgroup} onChange={(e) => setAthenaWorkgroup(e.target.value)} placeholder="primary" />
                      </label>
                      <label className="flex flex-col gap-1.5">
                        <span className="text-xs font-medium text-muted-foreground">Billing data region</span>
                        <input className={inputCls} value={finopsRegion} onChange={(e) => setFinopsRegion(e.target.value)} placeholder="eu-central-1" />
                      </label>
                    </div>
                  </section>
                </>
              )}
              {providerType === "azure" && (
                <>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Location (Azure)</span>
                    <input className={inputCls} value={region} onChange={(e) => setRegion(e.target.value)} placeholder="westeurope" />
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Subscription ID</span>
                    <input
                      className={inputCls}
                      value={azureSubscriptionId}
                      onChange={(e) => setAzureSubscriptionId(e.target.value)}
                      placeholder="00000000-0000-0000-0000-000000000000"
                    />
                  </label>
                  <section className="space-y-3 rounded-lg border border-border bg-muted/30 p-3">
                    <label className="flex items-center justify-between gap-3">
                      <span>
                        <span className="block text-xs font-medium text-muted-foreground">FinOps FOCUS export</span>
                        <span className="block text-[11px] text-muted-foreground">Connect Azure Cost Management exports to OPORD actual-cost mode.</span>
                      </span>
                      <input
                        type="checkbox"
                        checked={finopsEnabled}
                        onChange={(e) => setFinopsEnabled(e.target.checked)}
                        className="size-4 accent-primary"
                      />
                    </label>
                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                      <label className="flex flex-col gap-1.5">
                        <span className="text-xs font-medium text-muted-foreground">Export name</span>
                        <input className={inputCls} value={azureExportName} onChange={(e) => setAzureExportName(e.target.value)} placeholder="opord-focus-cost" />
                      </label>
                      <label className="flex flex-col gap-1.5">
                        <span className="text-xs font-medium text-muted-foreground">FOCUS version</span>
                        <input className={inputCls} value={azureFocusVersion} onChange={(e) => setAzureFocusVersion(e.target.value)} placeholder="1.2-preview" />
                      </label>
                    </div>
                    <label className="flex flex-col gap-1.5">
                      <span className="text-xs font-medium text-muted-foreground">Storage account</span>
                      <input className={inputCls} value={azureStorageAccount} onChange={(e) => setAzureStorageAccount(e.target.value)} placeholder="opordfinops424388" />
                    </label>
                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                      <label className="flex flex-col gap-1.5">
                        <span className="text-xs font-medium text-muted-foreground">Container</span>
                        <input className={inputCls} value={azureContainer} onChange={(e) => setAzureContainer(e.target.value)} placeholder="focus" />
                      </label>
                      <label className="flex flex-col gap-1.5">
                        <span className="text-xs font-medium text-muted-foreground">Directory</span>
                        <input className={inputCls} value={azureDirectory} onChange={(e) => setAzureDirectory(e.target.value)} placeholder="azure/focus" />
                      </label>
                    </div>
                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                      <label className="flex flex-col gap-1.5">
                        <span className="text-xs font-medium text-muted-foreground">Format</span>
                        <select className={inputCls} value={azureFormat} onChange={(e) => setAzureFormat(e.target.value)}>
                          <option value="csv">CSV</option>
                          <option value="parquet">Parquet</option>
                        </select>
                      </label>
                      <label className="flex flex-col gap-1.5">
                        <span className="text-xs font-medium text-muted-foreground">Compression</span>
                        <select className={inputCls} value={azureCompression} onChange={(e) => setAzureCompression(e.target.value)}>
                          <option value="gzip">Gzip</option>
                          <option value="snappy">Snappy</option>
                          <option value="none">None</option>
                        </select>
                      </label>
                    </div>
                  </section>
                </>
              )}
              {providerType === "gcp" && (
                <>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Project ID (GCP)</span>
                    <input className={inputCls} value={gcpProjectId} onChange={(e) => setGcpProjectId(e.target.value)} placeholder="my-gcp-project" />
                    <span className="text-[11px] text-muted-foreground">GCP project where OPORD creates resources. Credentials stay in OpenBao.</span>
                  </label>
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <label className="flex flex-col gap-1.5">
                      <span className="text-xs font-medium text-muted-foreground">Region (GCP)</span>
                      <input className={inputCls} value={region} onChange={(e) => setRegion(e.target.value)} placeholder="europe-west1" />
                    </label>
                    <label className="flex flex-col gap-1.5">
                      <span className="text-xs font-medium text-muted-foreground">Zone (GCP)</span>
                      <input className={inputCls} value={gcpZone} onChange={(e) => setGcpZone(e.target.value)} placeholder="europe-west1-b" />
                    </label>
                  </div>
                  <section className="space-y-3 rounded-lg border border-border bg-muted/30 p-3">
                    <label className="flex items-center justify-between gap-3">
                      <span>
                        <span className="block text-xs font-medium text-muted-foreground">FinOps FOCUS export</span>
                        <span className="block text-[11px] text-muted-foreground">Connect Google Cloud Billing BigQuery export and FOCUS view.</span>
                      </span>
                      <input
                        type="checkbox"
                        checked={finopsEnabled}
                        onChange={(e) => setFinopsEnabled(e.target.checked)}
                        className="size-4 accent-primary"
                      />
                    </label>
                    <label className="flex flex-col gap-1.5">
                      <span className="text-xs font-medium text-muted-foreground">Billing BigQuery project</span>
                      <input className={inputCls} value={gcpBillingProjectId} onChange={(e) => setGcpBillingProjectId(e.target.value)} placeholder="billing-project-id" />
                    </label>
                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                      <label className="flex flex-col gap-1.5">
                        <span className="text-xs font-medium text-muted-foreground">BigQuery dataset</span>
                        <input className={inputCls} value={gcpDataset} onChange={(e) => setGcpDataset(e.target.value)} placeholder="opord_billing" />
                      </label>
                      <label className="flex flex-col gap-1.5">
                        <span className="text-xs font-medium text-muted-foreground">FOCUS view</span>
                        <input className={inputCls} value={gcpFocusView} onChange={(e) => setGcpFocusView(e.target.value)} placeholder="focus_v1_0" />
                      </label>
                    </div>
                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                      <label className="flex flex-col gap-1.5">
                        <span className="text-xs font-medium text-muted-foreground">Detailed export table</span>
                        <input className={inputCls} value={gcpDetailedTable} onChange={(e) => setGcpDetailedTable(e.target.value)} placeholder="gcp_billing_export_resource_v1_..." />
                      </label>
                      <label className="flex flex-col gap-1.5">
                        <span className="text-xs font-medium text-muted-foreground">Pricing export table</span>
                        <input className={inputCls} value={gcpPricingTable} onChange={(e) => setGcpPricingTable(e.target.value)} placeholder="cloud_pricing_export" />
                      </label>
                    </div>
                  </section>
                </>
              )}
              {(providerType === "vsphere" || providerType === "proxmox") && (
                <label className="flex flex-col gap-1.5">
                  <span className="text-xs font-medium text-muted-foreground">
                    {providerType === "vsphere" ? "Server (vCenter FQDN/IP)" : "Server (Proxmox API URL)"}
                  </span>
                  <input
                    className={inputCls}
                    value={server}
                    onChange={(e) => setServer(e.target.value)}
                    placeholder={providerType === "vsphere" ? "vcenter.example.com" : "https://pve.example.com:8006/api2/json"}
                  />
                </label>
              )}
              {providerType === "vsphere" && (
                <label className="flex flex-col gap-1.5">
                  <span className="text-xs font-medium text-muted-foreground">Datacenter</span>
                  <input className={inputCls} value={datacenter} onChange={(e) => setDatacenter(e.target.value)} placeholder="dc-01" />
                </label>
              )}
              <label className="flex flex-col gap-1.5">
                <span className="text-xs font-medium text-muted-foreground">Advanced config (JSON)</span>
                <textarea
                  className="h-24 w-full rounded-lg border border-border bg-card p-2 font-mono text-xs text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  value={advancedJson}
                  onChange={(e) => setAdvancedJson(e.target.value)}
                  placeholder={
                    providerType === "aws"
                      ? '{ "subnet_ids": ["subnet-aaa","subnet-bbb"], "ou_id": "ou-xxxx" }'
                      : providerType === "azure"
                        ? '{ "aks_node_vm_size": "Standard_B2s" }'
                        : providerType === "gcp"
                          ? '{ "gcp_allow_ssh_from": ["10.0.0.0/8"], "gcs_location": "EU" }'
                          : '{ "datastore": "datastore-ssd", "network": "VM Network" }'
                  }
                  spellCheck={false}
                />
                <span className="text-[11px] text-muted-foreground">Merged into config (adds/overrides keys; others preserved). E.g. EKS subnet_ids, account ou_id.</span>
              </label>

              <div className="flex justify-end gap-2 pt-1">
                <button type="button" onClick={() => setEditing(false)} className={cn(button({ variant: "outline", size: "md" }))}>
                  Cancel
                </button>
                <button type="submit" disabled={busy} className={cn(button({ size: "md" }))}>
                  {busy && <Loader2 className="size-4 animate-spin" />}
                  Save
                </button>
              </div>
            </form>
          </div>,
          document.body,
        )}
    </div>
  );
}
