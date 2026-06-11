"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Check, Loader2, TriangleAlert } from "lucide-react";
import type { Provider } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { authHeaders } from "@/lib/client-auth";
import { DeployIntoField } from "@/components/deploy-into-field";

const API = "/bff";

type Ami = { id: string; name: string; description: string };

const presets = [
  { label: "Small", cpu: 2, ramGb: 4, diskGb: 40 },
  { label: "Medium", cpu: 4, ramGb: 8, diskGb: 80 },
  { label: "Large", cpu: 8, ramGb: 16, diskGb: 160 },
];

// Friendly auto-destroy choices. Keeps cloud test VMs from lingering and
// quietly accruing cost - the VM is torn down automatically after this long.
const ttlChoices = [
  { label: "No auto-destroy", hours: 0 },
  { label: "1 hour", hours: 1 },
  { label: "4 hours", hours: 4 },
  { label: "8 hours", hours: 8 },
  { label: "1 day", hours: 24 },
  { label: "3 days", hours: 72 },
];

// Common AWS regions and instance types for the cloud view's dropdowns. The
// region defaults to the provider's; instance type falls back to a free-text
// "Custom…" entry for anything not listed.
const awsRegions = [
  "us-east-1",
  "us-east-2",
  "us-west-1",
  "us-west-2",
  "eu-west-1",
  "eu-west-2",
  "eu-central-1",
  "eu-north-1",
  "ap-southeast-1",
  "ap-southeast-2",
  "ap-northeast-1",
  "ap-south-1",
  "ca-central-1",
  "sa-east-1",
];

const instanceTypes = [
  "t3.micro",
  "t3.small",
  "t3.medium",
  "t3.large",
  "t3a.medium",
  "t3a.large",
  "m5.large",
  "m5.xlarge",
  "c5.large",
  "c5.xlarge",
  "r5.large",
];

// Azure regions (Linux VMs); the location ends up in tofu's `location` variable.
const azureLocations = [
  "westeurope",
  "northeurope",
  "eastus",
  "eastus2",
  "westus2",
  "westus3",
  "centralus",
  "uksouth",
  "ukwest",
  "francecentral",
  "germanywestcentral",
  "swedencentral",
  "switzerlandnorth",
  "norwayeast",
  "southeastasia",
  "japaneast",
];

// Azure VM sizes; Standard_B1s is the cheapest Linux dev SKU.
const azureSizes = [
  "Standard_B1s",
  "Standard_B1ms",
  "Standard_B2s",
  "Standard_B2ms",
  "Standard_B4ms",
  "Standard_D2s_v5",
  "Standard_D4s_v5",
  "Standard_E2s_v5",
  "Standard_F2s_v2",
];

const gcpRegions = ["europe-west1", "europe-west4", "us-central1", "us-east1", "asia-southeast1"];

const gcpMachineTypes = ["e2-micro", "e2-small", "e2-medium", "e2-standard-2", "n2-standard-2", "n2-standard-4"];

const inputCls =
  "h-9 w-full rounded-lg border border-input bg-card px-3 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-xs font-medium text-muted-foreground">{label}</span>
      {children}
      {hint && <span className="text-[11px] text-muted-foreground">{hint}</span>}
    </label>
  );
}

export function NewVMForm({ providers, initialProvider }: { providers: Provider[]; initialProvider?: string }) {
  const router = useRouter();

  const presetProvider =
    initialProvider && providers.some((p) => p.name === initialProvider) ? initialProvider : undefined;

  const [name, setName] = useState("");
  const [environment, setEnvironment] = useState("dev");
  const [provider, setProvider] = useState(presetProvider ?? providers[0]?.name ?? "");
  const [template, setTemplate] = useState("");
  const [count, setCount] = useState(1);

  const [cpu, setCpu] = useState(2);
  const [ramGb, setRamGb] = useState(4);
  const [diskGb, setDiskGb] = useState(40);

  const initialRegion =
    providers.find((p) => p.name === (presetProvider ?? providers[0]?.name))?.region || "eu-central-1";
  const [instanceType, setInstanceType] = useState("t3.micro");
  const [customType, setCustomType] = useState(false);
  const [region, setRegion] = useState(initialRegion);
  const [publicIp, setPublicIp] = useState(false);

  // AMI picker (cloud): images are fetched live per provider+region. Falls back
  // to free-text entry when the list is empty or the call fails (e.g. no creds).
  const [images, setImages] = useState<Ami[]>([]);
  const [imagesLoading, setImagesLoading] = useState(false);
  const [imagesError, setImagesError] = useState(false);
  const [imagesErrorMsg, setImagesErrorMsg] = useState("");
  const [customAmi, setCustomAmi] = useState(false);
  const [amiSource, setAmiSource] = useState<"self" | "public">("self");

  const [ipStart, setIpStart] = useState("");
  const [gateway, setGateway] = useState("");
  const [dns, setDns] = useState("");

  const [ttlHours, setTtlHours] = useState(0);

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);

  // Cloud providers (AWS/Azure) size by instance type and have no static IP plan;
  // on-prem (vSphere/Proxmox) size by vCPU/RAM and need an IP range.
  const selectedType = providers.find((p) => p.name === provider)?.type;
  const isAws = selectedType === "aws";
  const isAzure = selectedType === "azure";
  const isGCP = selectedType === "gcp";
  const isCloud = isAws || isAzure || isGCP;
  // Azure + GCP VMs are SSH-key based and their tofu modules ship a default
  // Ubuntu image, so they show an SSH textarea instead of an image/AMI picker.
  const isSSHKeyCloud = isAzure || isGCP;
  const regionList = isAzure ? azureLocations : isGCP ? gcpRegions : awsRegions;
  const sizeList = isAzure ? azureSizes : isGCP ? gcpMachineTypes : instanceTypes;

  const [sshPublicKey, setSshPublicKey] = useState("");
  const [targetAccount, setTargetAccount] = useState("");

  // When the provider changes, reset the per-provider fields so a value from one
  // cloud doesn't leak into another - the bug where a GCP region (us-central1)
  // stayed selected after switching to AWS and broke DescribeImages
  // (ec2.us-central1.amazonaws.com: no such host), or an AWS t3.micro reached a
  // GCP/Azure machine_type. React's "adjust state during render" pattern: runs
  // before the AMI-fetch effect and avoids a set-state-in-effect lint.
  const [prevProvider, setPrevProvider] = useState(provider);
  if (provider !== prevProvider) {
    setPrevProvider(provider);
    const p = providers.find((x) => x.name === provider);
    setRegion(p?.region || "eu-central-1");
    setTargetAccount("");
    setInstanceType(p?.type === "azure" ? "Standard_B1s" : p?.type === "gcp" ? "e2-micro" : "t3.micro");
    setCustomType(false);
  }
  const activePreset = presets.find((p) => p.cpu === cpu && p.ramGb === ramGb && p.diskGb === diskGb);

  // Fetch AMIs whenever the AWS provider or region changes. setState lives inside
  // the async IIFE (not the effect body) to satisfy react-hooks/set-state-in-effect.
  useEffect(() => {
    if (!isAws || !provider) return;
    let cancelled = false;
    void (async () => {
      setImagesLoading(true);
      setImagesError(false);
      setImagesErrorMsg("");
      try {
        const res = await fetch(
          `${API}/api/v1/providers/${encodeURIComponent(provider)}/images?region=${encodeURIComponent(region)}&owner=${amiSource}`,
          { headers: authHeaders() },
        );
        if (!res.ok) {
          const body = (await res.json().catch(() => ({}))) as { error?: string };
          throw new Error(body.error || `request failed (${res.status})`);
        }
        const data = (await res.json()) as Ami[];
        if (!cancelled) setImages(Array.isArray(data) ? data : []);
      } catch (err) {
        if (!cancelled) {
          setImages([]);
          setImagesError(true);
          setImagesErrorMsg(err instanceof Error ? err.message : String(err));
        }
      } finally {
        if (!cancelled) setImagesLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [isAws, provider, region, amiSource]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    setOk(null);
    try {
      const spec: Record<string, unknown> = {
        template,
        count,
        disk_gb: diskGb,
        ssh_user: "debian",
        ttl_hours: ttlHours,
      };
      if (isCloud) {
        spec.instance_type = instanceType;
        spec.region = region;
        spec.public_ip = publicIp;
        // Deploy into a OPORD-managed account (ADR-0013) instead of the provider's
        // default - GCP project / Azure subscription / AWS member account (cross-account
        // AssumeRole). The region must comply with that account's policies.
        if ((isAws || isGCP || isAzure) && targetAccount.trim()) spec.target_account = targetAccount.trim();
        if (isSSHKeyCloud) {
          // Azure requires an SSH key; GCP it's optional (OS Login/IAP). Their
          // modules default the image, so don't send the AWS AMI in `template`.
          if (sshPublicKey.trim()) spec.ssh_public_key = sshPublicKey.trim();
          delete spec.template;
        }
      } else {
        spec.cpu = cpu;
        spec.memory_mb = ramGb * 1024;
        spec.ip_start = ipStart;
        spec.netmask = "255.255.255.0";
        spec.gateway = gateway;
        spec.dns_servers = dns ? dns.split(",").map((s) => s.trim()).filter(Boolean) : [];
        spec.dns_suffix = "local";
      }

      const res = await fetch(`${API}/api/v1/vms`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ name, environment, provider, spec }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `request failed (${res.status})`);
        return;
      }
      setOk(`Virtual machine "${data.name}" created - status ${data.status}.`);
      router.refresh();
      setTimeout(() => router.push("/vms"), 900);
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
            <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} placeholder="my-vm" required />
          </Field>
          <Field label="Environment">
            <select className={inputCls} value={environment} onChange={(e) => setEnvironment(e.target.value)}>
              <option value="dev">dev</option>
              <option value="staging">staging</option>
              <option value="production">production</option>
            </select>
          </Field>
          <Field label="Provider" hint="Where the VM is created.">
            <select className={inputCls} value={provider} onChange={(e) => setProvider(e.target.value)} required>
              {providers.length === 0 && <option value="">no providers registered</option>}
              {providers.map((p) => (
                <option key={p.id} value={p.name}>
                  {p.name} ({p.type})
                </option>
              ))}
            </select>
          </Field>
          {!isCloud ? (
            <Field label="Template" hint="Golden image to clone.">
              <input
                className={inputCls}
                value={template}
                onChange={(e) => setTemplate(e.target.value)}
                placeholder="debian-12-base"
                required
              />
            </Field>
          ) : isSSHKeyCloud ? (
            <Field
              label={isAzure ? "SSH public key" : "SSH public key (optional)"}
              hint={
                isAzure
                  ? "Linux Azure VMs are SSH-only (no password auth). Paste your OpenSSH public key."
                  : "GCP VM runs a default Ubuntu image. Add an SSH key for access, or leave blank to use OS Login / IAP."
              }
            >
              <textarea
                className="h-20 w-full rounded-lg border border-border bg-card p-2 font-mono text-xs text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                value={sshPublicKey}
                onChange={(e) => setSshPublicKey(e.target.value)}
                placeholder="ssh-ed25519 AAAA… user@host"
                spellCheck={false}
                required={isAzure}
              />
            </Field>
          ) : (
            <div className="flex flex-col gap-1.5">
              <div className="flex items-center justify-between gap-2">
                <span className="text-xs font-medium text-muted-foreground">AMI</span>
                <div className="flex rounded-lg border border-border p-0.5 text-[11px]">
                  {(
                    [
                      ["self", "My AMIs"],
                      ["public", "Public OS"],
                    ] as const
                  ).map(([src, label]) => (
                    <button
                      key={src}
                      type="button"
                      aria-pressed={amiSource === src}
                      onClick={() => setAmiSource(src)}
                      className={cn(
                        "rounded-md px-2 py-0.5 font-medium transition-colors",
                        amiSource === src ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:text-foreground",
                      )}
                    >
                      {label}
                    </button>
                  ))}
                </div>
              </div>

              {imagesLoading ? (
                <select className={inputCls} disabled aria-label="AMI">
                  <option>Loading AMIs…</option>
                </select>
              ) : images.length === 0 ? (
                <input
                  className={inputCls}
                  value={template}
                  onChange={(e) => setTemplate(e.target.value)}
                  placeholder="ami-0abc123def456"
                  aria-label="AMI"
                  required
                />
              ) : (
                <>
                  <select
                    className={inputCls}
                    aria-label="AMI"
                    value={customAmi ? "__custom" : images.some((im) => im.id === template) ? template : ""}
                    onChange={(e) => {
                      if (e.target.value === "__custom") {
                        setCustomAmi(true);
                      } else {
                        setCustomAmi(false);
                        setTemplate(e.target.value);
                      }
                    }}
                    required
                  >
                    <option value="" disabled>
                      Select an AMI…
                    </option>
                    {images.map((im) => (
                      <option key={im.id} value={im.id}>
                        {im.id}
                        {im.name ? ` - ${im.name}` : ""}
                      </option>
                    ))}
                    <option value="__custom">Custom / enter ID…</option>
                  </select>
                  {customAmi && (
                    <input
                      className={inputCls}
                      value={template}
                      onChange={(e) => setTemplate(e.target.value)}
                      placeholder="ami-0abc123def456"
                      aria-label="AMI ID"
                      required
                    />
                  )}
                </>
              )}

              <span className="text-[11px] text-muted-foreground">
                {imagesError
                  ? `Couldn't list AMIs - ${imagesErrorMsg || "check AWS creds / region"}. Enter the ID manually.`
                  : imagesLoading
                    ? "Loading AMIs from AWS…"
                    : images.length === 0
                      ? amiSource === "public"
                        ? `No public OS images matched in ${region}.`
                        : "No AMIs in your account here - enter an ID manually."
                      : `${images.length} ${amiSource === "public" ? "public OS image" : "AMI"}(s) in ${region}, newest first.`}
              </span>
            </div>
          )}
          <DeployIntoField provider={provider} providerType={selectedType} value={targetAccount} onChange={setTargetAccount} />
          <Field label="How many" hint="Number of identical VMs.">
            <input type="number" min={1} className={inputCls} value={count} onChange={(e) => setCount(Math.max(1, +e.target.value))} />
          </Field>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Compute</CardTitle>
        </CardHeader>
        {isCloud ? (
          <CardContent className="space-y-4">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <Field
                label={isAzure ? "Location" : "Region"}
                hint={
                  isAzure
                    ? "Azure region (location). Defaults to the provider's location."
                    : isGCP
                      ? "GCP region. Defaults to the provider's region."
                      : "AWS region the VM launches in. Defaults to the provider's region."
                }
              >
                <select className={inputCls} value={region} onChange={(e) => setRegion(e.target.value)}>
                  {regionList
                    .concat(regionList.includes(region) ? [] : [region])
                    .map((r) => (
                      <option key={r} value={r}>
                        {r}
                      </option>
                    ))}
                </select>
              </Field>
              <Field label={isAzure ? "VM size" : isGCP ? "Machine type" : "Instance type"} hint="Pick a common size, or Custom… for anything else.">
                <select
                  className={inputCls}
                  value={
                    customType
                      ? "__custom"
                      : sizeList.includes(instanceType)
                        ? instanceType
                        : "__custom"
                  }
                  onChange={(e) => {
                    if (e.target.value === "__custom") {
                      setCustomType(true);
                    } else {
                      setCustomType(false);
                      setInstanceType(e.target.value);
                    }
                  }}
                >
                  {sizeList.map((t) => (
                    <option key={t} value={t}>
                      {t}
                    </option>
                  ))}
                  <option value="__custom">Custom…</option>
                </select>
                {customType && (
                  <input
                    className={inputCls}
                    value={instanceType}
                    onChange={(e) => setInstanceType(e.target.value)}
                    placeholder="e.g. m6i.2xlarge"
                    aria-label="Instance type"
                    required
                    autoFocus
                  />
                )}
              </Field>
              <Field label="Root disk (GB)">
                <input type="number" min={1} className={inputCls} value={diskGb} onChange={(e) => setDiskGb(Math.max(1, +e.target.value))} />
              </Field>
            </div>
            <label className="flex items-center gap-2 text-sm text-foreground">
              <input type="checkbox" className="size-4 rounded border-border" checked={publicIp} onChange={(e) => setPublicIp(e.target.checked)} />
              Assign a public IP address
            </label>
          </CardContent>
        ) : (
          <CardContent className="space-y-4">
            <div className="flex flex-wrap gap-2">
              {presets.map((p) => (
                <button
                  key={p.label}
                  type="button"
                  aria-pressed={activePreset?.label === p.label}
                  onClick={() => {
                    setCpu(p.cpu);
                    setRamGb(p.ramGb);
                    setDiskGb(p.diskGb);
                  }}
                  className={cn(
                    "rounded-lg border px-3 py-2 text-left text-sm transition-colors",
                    activePreset?.label === p.label
                      ? "border-primary bg-primary/10 text-primary"
                      : "border-border hover:bg-muted",
                  )}
                >
                  <div className="font-medium">{p.label}</div>
                  <div className="text-xs text-muted-foreground">
                    {p.cpu} vCPU · {p.ramGb} GB · {p.diskGb} GB
                  </div>
                </button>
              ))}
            </div>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
              <Field label="vCPU">
                <input type="number" min={1} className={inputCls} value={cpu} onChange={(e) => setCpu(Math.max(1, +e.target.value))} />
              </Field>
              <Field label="Memory (GB)">
                <input type="number" min={1} className={inputCls} value={ramGb} onChange={(e) => setRamGb(Math.max(1, +e.target.value))} />
              </Field>
              <Field label="Disk (GB)">
                <input type="number" min={1} className={inputCls} value={diskGb} onChange={(e) => setDiskGb(Math.max(1, +e.target.value))} />
              </Field>
            </div>
          </CardContent>
        )}
      </Card>

      {!isCloud && (
        <Card>
          <CardHeader>
            <CardTitle>Network</CardTitle>
          </CardHeader>
          <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <Field label="First IP" hint="VMs increment from here.">
              <input className={inputCls} value={ipStart} onChange={(e) => setIpStart(e.target.value)} placeholder="10.0.0.10" required />
            </Field>
            <Field label="Gateway">
              <input className={inputCls} value={gateway} onChange={(e) => setGateway(e.target.value)} placeholder="10.0.0.1" required />
            </Field>
            <Field label="DNS servers" hint="Comma-separated.">
              <input className={inputCls} value={dns} onChange={(e) => setDns(e.target.value)} placeholder="192.168.2.3" />
            </Field>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Lifecycle</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field
            label="Auto-destroy after"
            hint="OPORD tears the VM down automatically once this elapses - handy so test VMs don't linger and cost money."
          >
            <select className={inputCls} value={ttlHours} onChange={(e) => setTtlHours(+e.target.value)}>
              {ttlChoices.map((c) => (
                <option key={c.hours} value={c.hours}>
                  {c.label}
                </option>
              ))}
            </select>
          </Field>
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <button type="submit" disabled={submitting} className={cn(button({ size: "md" }), submitting && "opacity-70")}>
          {submitting && <Loader2 className="size-4 animate-spin" />}
          Create virtual machine{count > 1 ? `s (${count})` : ""}
        </button>
        <a href="/vms" className={button({ variant: "outline", size: "md" })}>
          Cancel
        </a>
      </div>
    </form>
  );
}
