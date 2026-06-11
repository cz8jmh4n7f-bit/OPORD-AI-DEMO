"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Check, ChevronDown, ChevronRight, Loader2, TriangleAlert } from "lucide-react";
import type { Provider } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { authHeaders } from "@/lib/client-auth";

const API = "/bff";
const inputCls =
  "h-9 w-full rounded-lg border border-border bg-card px-3 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring";

type BillingScope = { id: string; name: string; type: string };

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-xs font-medium text-muted-foreground">{label}</span>
      {children}
      {hint && <span className="text-[11px] text-muted-foreground">{hint}</span>}
    </label>
  );
}

// ModeCard is the big adopt-vs-create choice (Azure). Plain language up front so a
// non-technical user picks the path before seeing any expert fields.
function ModeCard({
  selected,
  title,
  desc,
  onClick,
}: {
  selected: boolean;
  title: string;
  desc: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "flex flex-1 flex-col gap-1 rounded-xl border p-3 text-left transition",
        selected ? "border-primary bg-primary/5 ring-2 ring-ring" : "border-border hover:border-primary/50",
      )}
    >
      <span className="text-sm font-medium text-foreground">{title}</span>
      <span className="text-[11px] text-muted-foreground">{desc}</span>
    </button>
  );
}

const azureRegions = ["westeurope", "northeurope", "eastus", "eastus2", "westus2", "uksouth", "francecentral", "swedencentral"];

export function NewAccountForm({ providers, initialProvider }: { providers: Provider[]; initialProvider?: string }) {
  const router = useRouter();
  const preset = initialProvider && providers.some((p) => p.name === initialProvider) ? initialProvider : undefined;

  const [name, setName] = useState("");
  const [environment, setEnvironment] = useState("dev");
  const [provider, setProvider] = useState(preset ?? providers.find((p) => p.type === "aws")?.name ?? providers[0]?.name ?? "");

  // Shared identity fields.
  const [csaId, setCsaId] = useState("");
  const [cloudName, setCloudName] = useState("prod");
  const [owner, setOwner] = useState("");

  // AWS-specific.
  const [email, setEmail] = useState("");
  const [createVpc, setCreateVpc] = useState(true);
  const [vpcRegion, setVpcRegion] = useState("eu-central-1");
  const [budget, setBudget] = useState(500);

  // Azure-specific (ADR-0009 subscription factory).
  const [azureMode, setAzureMode] = useState<"adopt" | "create">("adopt");
  const [azureSubscriptionId, setAzureSubscriptionId] = useState("");
  const [azureBillingScope, setAzureBillingScope] = useState("");
  const [azureLocation, setAzureLocation] = useState("westeurope");
  const [azureVnetCidr, setAzureVnetCidr] = useState("10.20.0.0/22");
  const [azureAllowInbound, setAzureAllowInbound] = useState("0.0.0.0/0");
  const [azureSkipGroups, setAzureSkipGroups] = useState(false);

  // GCP-specific (ADR-0011 project factory).
  const [gcpRegion, setGcpRegion] = useState("europe-west1");
  const [gcpVpcCidr, setGcpVpcCidr] = useState("");
  const [gcpAllowInbound, setGcpAllowInbound] = useState("0.0.0.0/0");

  // Azure billing-profile picker (Phase 2): fetched live; falls back to a pasted URI.
  const [billingScopes, setBillingScopes] = useState<BillingScope[]>([]);
  const [billingLoading, setBillingLoading] = useState(false);
  const [billingError, setBillingError] = useState("");
  const [billingManual, setBillingManual] = useState(false);

  const [showAdvanced, setShowAdvanced] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);

  const selectedType = providers.find((p) => p.name === provider)?.type;
  const isAzure = selectedType === "azure";
  const isGcp = selectedType === "gcp";
  const isAws = !isAzure && !isGcp;

  const headline = isAzure ? "Azure subscription" : isGcp ? "GCP project" : "AWS member account";

  // Fetch Azure billing profiles when create mode is active. setState lives inside
  // the async IIFE (not the effect body) to satisfy react-hooks/set-state-in-effect.
  useEffect(() => {
    if (!isAzure || azureMode !== "create" || !provider) return;
    let cancelled = false;
    void (async () => {
      setBillingLoading(true);
      setBillingError("");
      try {
        const res = await fetch(`${API}/api/v1/providers/${encodeURIComponent(provider)}/billing-scopes`, {
          headers: authHeaders(),
        });
        if (!res.ok) {
          const body = (await res.json().catch(() => ({}))) as { error?: string };
          throw new Error(body.error || `request failed (${res.status})`);
        }
        const data = (await res.json()) as BillingScope[];
        if (!cancelled) setBillingScopes(Array.isArray(data) ? data : []);
      } catch (err) {
        if (!cancelled) {
          setBillingScopes([]);
          setBillingError(err instanceof Error ? err.message : String(err));
        }
      } finally {
        if (!cancelled) setBillingLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [isAzure, azureMode, provider]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setOk(null);

    if (isAzure && azureMode === "create" && !azureBillingScope.trim()) {
      setError("Pick a billing profile (or paste a billing scope) for create mode.");
      return;
    }

    setSubmitting(true);
    try {
      // Build the provider-appropriate spec. The backend AccountSpec carries
      // AWS/Azure/GCP fields; we only send the relevant ones.
      const spec: Record<string, unknown> = {
        csa_id: csaId,
        cloud_name: cloudName,
        owner,
      };
      if (isAzure) {
        spec.azure_mode = azureMode;
        if (azureMode === "adopt") spec.azure_subscription_id = azureSubscriptionId.trim();
        if (azureMode === "create") spec.azure_billing_scope_id = azureBillingScope.trim();
        spec.azure_location = azureLocation;
        spec.azure_allowed_locations = ["westeurope", "northeurope"];
        if (azureVnetCidr.trim()) spec.azure_vnet_cidr = azureVnetCidr.trim();
        spec.azure_allow_inbound_cidrs = azureAllowInbound
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean);
        if (azureSkipGroups) spec.skip = { azure_rbac_groups: true };
      } else if (isGcp) {
        spec.gcp_mode = "create";
        spec.create_vpc = createVpc;
        spec.vpc_region = gcpRegion;
        if (gcpVpcCidr.trim()) spec.vpc_cidr = gcpVpcCidr.trim();
        spec.gcp_allow_inbound_cidrs = gcpAllowInbound
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean);
      } else {
        spec.email = email;
        spec.create_vpc = createVpc;
        spec.vpc_region = vpcRegion;
        spec.monthly_budget_usd = budget;
      }

      // Display name defaults to the Project ID when left blank.
      const displayName = name.trim() || csaId.trim();

      const res = await fetch(`${API}/api/v1/accounts`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ name: displayName, environment, provider, spec }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `request failed (${res.status})`);
        return;
      }
      setOk(`Account "${data.name}" registered - status ${data.status}.`);
      router.refresh();
      setTimeout(() => router.push("/accounts"), 900);
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
          <CardTitle>{headline} - governed baseline</CardTitle>
          <p className="text-sm text-muted-foreground">
            {isAzure
              ? "Create a new subscription or govern an existing one, then apply the baseline (RBAC, secure network, logging, policy)."
              : isGcp
                ? "Create a new GCP project under your org folder, then apply the baseline (APIs, IAM, secure VPC, org policy, security)."
                : "Create a new member account under your AWS Organization, then apply the baseline (baseline, access, secure VPC, security)."}
          </p>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          {/* Provider first - it's the connected org/tenant this lands under. */}
          <Field
            label="Provider (connected org/tenant)"
            hint={
              isAzure
                ? "Your connected Azure tenant. (Set up once by an admin.)"
                : isGcp
                  ? "Your connected GCP org. (Set up once by an admin.)"
                  : "Your connected AWS Organization. (Set up once by an admin.)"
            }
          >
            <select className={inputCls} value={provider} onChange={(e) => setProvider(e.target.value)} required>
              {providers.length === 0 && <option value="">no providers registered</option>}
              {providers.map((p) => (
                <option key={p.id} value={p.name}>
                  {p.name} ({p.type})
                </option>
              ))}
            </select>
          </Field>

          {/* Azure: lead with the new-vs-existing decision, in plain language. */}
          {isAzure && (
            <div className="flex flex-col gap-1.5 sm:col-span-2">
              <span className="text-xs font-medium text-muted-foreground">What do you want to do?</span>
              <div className="flex flex-col gap-2 sm:flex-row">
                <ModeCard
                  selected={azureMode === "create"}
                  title="🆕 Create a new subscription"
                  desc="OPORD creates a brand-new Azure subscription and applies the baseline."
                  onClick={() => setAzureMode("create")}
                />
                <ModeCard
                  selected={azureMode === "adopt"}
                  title="📥 Use an existing subscription"
                  desc="Apply the governed baseline to a subscription you already have."
                  onClick={() => setAzureMode("adopt")}
                />
              </div>
            </div>
          )}

          {/* Basics - plain, always shown. */}
          <Field label="Project ID" hint="Short unique id - used in all resource names (e.g. alpha001).">
            <input className={inputCls} value={csaId} onChange={(e) => setCsaId(e.target.value)} placeholder="alpha001" required />
          </Field>
          <Field label="Owner" hint={isAzure ? "Email or username tagged as owner." : isGcp ? "Email or username tagged as owner." : "Username (no domain)."}>
            <input className={inputCls} value={owner} onChange={(e) => setOwner(e.target.value)} placeholder="alice" required />
          </Field>
          <Field label="Environment">
            <select className={inputCls} value={environment} onChange={(e) => setEnvironment(e.target.value)}>
              <option value="dev">dev</option>
              <option value="staging">staging</option>
              <option value="production">production</option>
            </select>
          </Field>

          {isAzure && (
            <Field label="Location" hint="Default region for the base resource groups.">
              <select className={inputCls} value={azureLocation} onChange={(e) => setAzureLocation(e.target.value)}>
                {azureRegions.map((r) => (
                  <option key={r} value={r}>
                    {r}
                  </option>
                ))}
              </select>
            </Field>
          )}

          {/* Azure adopt to subscription id; create to billing-profile picker. */}
          {isAzure && azureMode === "adopt" && (
            <Field label="Subscription ID" hint="GUID of the existing subscription the SP owns (Azure Portal to Subscriptions).">
              <input
                className={inputCls}
                value={azureSubscriptionId}
                onChange={(e) => setAzureSubscriptionId(e.target.value)}
                placeholder="00000000-0000-0000-0000-000000000000"
                required
              />
            </Field>
          )}
          {isAzure && azureMode === "create" && (
            <div className="flex flex-col gap-1.5 sm:col-span-2">
              <span className="text-xs font-medium text-muted-foreground">Billing profile (where Azure bills this subscription)</span>
              {billingLoading ? (
                <div className="flex h-9 items-center gap-2 rounded-lg border border-border bg-card px-3 text-sm text-muted-foreground">
                  <Loader2 className="size-4 animate-spin" /> Loading billing profiles…
                </div>
              ) : billingScopes.length > 0 && !billingManual ? (
                <select
                  className={inputCls}
                  value={azureBillingScope}
                  onChange={(e) => setAzureBillingScope(e.target.value)}
                >
                  <option value="">Select a billing profile…</option>
                  {billingScopes.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
                </select>
              ) : (
                <input
                  className={inputCls}
                  value={azureBillingScope}
                  onChange={(e) => setAzureBillingScope(e.target.value)}
                  placeholder="/providers/Microsoft.Billing/billingAccounts/…/invoiceSections/…"
                />
              )}
              <span className="text-[11px] text-muted-foreground">
                {billingScopes.length > 0 && !billingManual ? (
                  <>
                    Picked from your tenant.{" "}
                    <button type="button" className="underline hover:text-foreground" onClick={() => setBillingManual(true)}>
                      Paste a scope manually
                    </button>
                  </>
                ) : billingError ? (
                  <>Couldn’t list billing profiles ({billingError}). Paste the invoice-section URI from Azure Portal to Cost Management + Billing.</>
                ) : billingScopes.length === 0 ? (
                  <>No billing profiles found - paste the invoice-section URI (Azure Portal to Cost Management + Billing to Billing profiles).</>
                ) : (
                  <button type="button" className="underline hover:text-foreground" onClick={() => setBillingManual(false)}>
                    Back to the picker
                  </button>
                )}
              </span>
            </div>
          )}

          {/* AWS - root email is essential, so it stays primary. */}
          {isAws && (
            <Field label="Root email" hint="Unique account email (use plus-addressing or a catch-all).">
              <input className={inputCls} value={email} onChange={(e) => setEmail(e.target.value)} placeholder="aws+alpha001-prod@example.com" required />
            </Field>
          )}

          {/* GCP - region for the secure VPC subnets. */}
          {isGcp && (
            <Field label="Region" hint="Region for the secure VPC subnets (folder/billing come from the provider).">
              <input className={inputCls} value={gcpRegion} onChange={(e) => setGcpRegion(e.target.value)} placeholder="europe-west1" disabled={!createVpc} />
            </Field>
          )}

          {/* Advanced - expert knobs, collapsed, with sensible defaults. */}
          <div className="sm:col-span-2">
            <button
              type="button"
              onClick={() => setShowAdvanced((v) => !v)}
              className="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-foreground"
            >
              {showAdvanced ? <ChevronDown className="size-4" /> : <ChevronRight className="size-4" />}
              Advanced {isAzure ? "(network, access, naming)" : isGcp ? "(network, naming)" : "(budget, network, naming)"}
            </button>
          </div>

          {showAdvanced && (
            <>
              <Field label="Display name" hint="Shown in OPORD. Defaults to the Project ID.">
                <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} placeholder={csaId || "team-alpha-prod"} />
              </Field>
              <Field label="Cloud name" hint="Environment label baked into the resource name.">
                <select className={inputCls} value={cloudName} onChange={(e) => setCloudName(e.target.value)}>
                  <option value="prod">prod</option>
                  <option value="stage">stage</option>
                  <option value="dev">dev</option>
                </select>
              </Field>

              {isAws && (
                <>
                  <Field label="Monthly budget (USD)">
                    <input type="number" min={0} className={inputCls} value={budget} onChange={(e) => setBudget(Number(e.target.value))} />
                  </Field>
                  <Field label="VPC region" hint="Region for the secure VPC.">
                    <input className={inputCls} value={vpcRegion} onChange={(e) => setVpcRegion(e.target.value)} placeholder="eu-central-1" disabled={!createVpc} />
                  </Field>
                  <label className="flex items-center gap-2 self-end pb-2 sm:col-span-2">
                    <input type="checkbox" checked={createVpc} onChange={(e) => setCreateVpc(e.target.checked)} className="size-4 rounded border-border" />
                    <span className="text-sm text-foreground">Create secure VPC (/22 from the CIDR pool)</span>
                  </label>
                </>
              )}

              {isGcp && (
                <>
                  <Field label="VPC range (/22)" hint="Leave empty to let OPORD allocate from the pool.">
                    <input className={inputCls} value={gcpVpcCidr} onChange={(e) => setGcpVpcCidr(e.target.value)} placeholder="(auto-allocated)" disabled={!createVpc} />
                  </Field>
                  <Field label="Allowed inbound IPs" hint="Comma-separated trusted sources for the firewall.">
                    <input className={inputCls} value={gcpAllowInbound} onChange={(e) => setGcpAllowInbound(e.target.value)} placeholder="0.0.0.0/0" disabled={!createVpc} />
                  </Field>
                  <label className="flex items-center gap-2 self-end pb-2 sm:col-span-2">
                    <input type="checkbox" checked={createVpc} onChange={(e) => setCreateVpc(e.target.checked)} className="size-4 rounded border-border" />
                    <span className="text-sm text-foreground">Create secure VPC (/22 from the pool)</span>
                  </label>
                </>
              )}

              {isAzure && (
                <>
                  <Field label="Private network range (/22)" hint="Carved into three /24 subnets. Empty = skip the network layer.">
                    <input className={inputCls} value={azureVnetCidr} onChange={(e) => setAzureVnetCidr(e.target.value)} placeholder="10.20.0.0/22" />
                  </Field>
                  <Field label="Allowed inbound IPs" hint="Comma-separated. 0.0.0.0/0 = dev only.">
                    <input className={inputCls} value={azureAllowInbound} onChange={(e) => setAzureAllowInbound(e.target.value)} placeholder="0.0.0.0/0" />
                  </Field>
                  <label className="flex items-center gap-2 self-end pb-2 sm:col-span-2">
                    <input type="checkbox" checked={azureSkipGroups} onChange={(e) => setAzureSkipGroups(e.target.checked)} className="size-4 rounded border-border" />
                    <span className="text-sm text-foreground">
                      Skip Entra group creation (use if the connected SP lacks Groups Administrator - role definitions still created)
                    </span>
                  </label>
                </>
              )}
            </>
          )}
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <button type="submit" disabled={submitting} className={cn(button({ size: "md" }), submitting && "opacity-70")}>
          {submitting && <Loader2 className="size-4 animate-spin" />}
          {isAzure ? (azureMode === "create" ? "Create subscription" : "Govern subscription") : isGcp ? "Create project" : "Create account"}
        </button>
        <Link href="/accounts" className={button({ variant: "outline", size: "md" })}>
          Cancel
        </Link>
      </div>
    </form>
  );
}
