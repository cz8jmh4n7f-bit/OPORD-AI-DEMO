"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { createPortal } from "react-dom";
import { Loader2, Plus } from "lucide-react";
import type { ProviderType } from "@/lib/types";
import { button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { authHeaders } from "@/lib/client-auth";
import { useToast } from "@/components/ui/toast";

const API = "/bff";

const inputCls =
  "h-9 w-full rounded-lg border border-input bg-card px-3 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";

// AddProviderButton registers a new infrastructure backend (POST /providers).
// Which connection fields show depends on the selected type; credentials never
// go here - only a secret-ref pointing at an OpenBao KV path. Mirrors the Edit
// modal in provider-actions.tsx.
export function AddProviderButton() {
  const router = useRouter();
  const { toast } = useToast();

  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);

  const [name, setName] = useState("");
  const [type, setType] = useState<ProviderType>("vsphere");
  const [secretRef, setSecretRef] = useState("");

  // vSphere / Proxmox connection (some fields shared, some type-specific).
  const [server, setServer] = useState("");
  const [datacenter, setDatacenter] = useState("");
  const [computeCluster, setComputeCluster] = useState("");
  const [datastore, setDatastore] = useState("");
  const [network, setNetwork] = useState("");
  const [folder, setFolder] = useState("");
  const [node, setNode] = useState("");
  const [templateVmid, setTemplateVmid] = useState("");
  // AWS
  const [region, setRegion] = useState("");
  const [subnetId, setSubnetId] = useState("");
  const [subnetIds, setSubnetIds] = useState("");
  const [securityGroupIds, setSecurityGroupIds] = useState("");
  const [keyName, setKeyName] = useState("");
  const [ouId, setOuId] = useState("");
  const [advancedJson, setAdvancedJson] = useState("");
  // Azure
  const [subscriptionId, setSubscriptionId] = useState("");
  const [azureLocation, setAzureLocation] = useState("");
  // GCP
  const [projectId, setProjectId] = useState("");
  const [gcpZone, setGcpZone] = useState("");

  function reset() {
    setName("");
    setType("vsphere");
    setSecretRef("");
    setServer("");
    setDatacenter("");
    setComputeCluster("");
    setDatastore("");
    setNetwork("");
    setFolder("");
    setNode("");
    setTemplateVmid("");
    setRegion("");
    setSubnetId("");
    setSubnetIds("");
    setSecurityGroupIds("");
    setKeyName("");
    setOuId("");
    setAdvancedJson("");
    setSubscriptionId("");
    setAzureLocation("");
  }

  // buildConfig assembles only the keys the selected backend reads, dropping
  // blanks so we never store empty noise (the backend tolerates a sparse config).
  // buildConfig assembles the keys the selected backend reads. May throw if the
  // AWS advanced-config JSON is invalid (caught in save()).
  function buildConfig(): Record<string, unknown> {
    let c: Record<string, unknown> = {};
    const put = (k: string, v: string) => {
      if (v.trim()) c[k] = v.trim();
    };
    const csv = (s: string) => s.split(",").map((x) => x.trim()).filter(Boolean);
    if (type === "vsphere") {
      put("server", server);
      put("datacenter", datacenter);
      put("cluster", computeCluster);
      put("datastore", datastore);
      put("network", network);
      put("folder", folder);
      c.allow_unverified_ssl = true;
    } else if (type === "proxmox") {
      put("server", server);
      put("node", node);
      put("datastore", datastore);
      put("network", network);
      put("template_vmid", templateVmid);
    } else if (type === "aws") {
      // Advanced JSON first (any extra keys: saml_metadata_document, node_instance_type, …);
      // typed fields below override it.
      if (advancedJson.trim()) c = { ...JSON.parse(advancedJson) };
      put("region", region);
      put("subnet_id", subnetId);
      put("key_name", keyName);
      put("ou_id", ouId);
      if (subnetIds.trim()) c.subnet_ids = csv(subnetIds);
      if (securityGroupIds.trim()) c.security_group_ids = csv(securityGroupIds);
    } else if (type === "azure") {
      put("subscription_id", subscriptionId);
      put("location", azureLocation);
    } else if (type === "gcp") {
      put("project_id", projectId);
      put("region", region);
      put("zone", gcpZone);
    }
    return c;
  }

  async function save(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) {
      toast({ variant: "error", title: "Name is required" });
      return;
    }
    let config: Record<string, unknown>;
    try {
      config = buildConfig();
    } catch {
      toast({ variant: "error", title: "Advanced config must be valid JSON" });
      return;
    }
    setBusy(true);
    try {
      const res = await fetch(`${API}/api/v1/providers`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ name: name.trim(), type, secretRef: secretRef.trim(), config }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        toast({ variant: "error", title: "Add failed", description: data.error ?? `Request failed (${res.status})` });
        return;
      }
      toast({ variant: "success", title: `Provider “${name.trim()}” registered` });
      setOpen(false);
      reset();
      router.refresh();
    } catch (err) {
      toast({ variant: "error", title: "Add failed", description: String(err) });
    } finally {
      setBusy(false);
    }
  }

  const onprem = type === "vsphere" || type === "proxmox";

  return (
    <>
      <button type="button" onClick={() => setOpen(true)} className={cn(button({ size: "md" }))}>
        <Plus className="size-4" />
        Add provider
      </button>

      {open &&
        typeof document !== "undefined" &&
        createPortal(
          <div className="fixed inset-0 z-[70] flex items-center justify-center p-4" role="dialog" aria-modal="true">
            <div className="absolute inset-0 bg-black/50" onClick={() => !busy && setOpen(false)} />
            <form
              onSubmit={save}
              className="relative max-h-[90vh] w-full max-w-md space-y-4 overflow-y-auto rounded-xl border border-border bg-card p-5 shadow-xl"
            >
              <div>
                <h2 className="text-base font-semibold text-foreground">Add provider</h2>
                <p className="mt-1 text-xs text-muted-foreground">
                  Register an infrastructure backend. Credentials are read from OpenBao at run time - only a secret-ref
                  is stored here.
                </p>
              </div>

              <label className="flex flex-col gap-1.5">
                <span className="text-xs font-medium text-muted-foreground">Name</span>
                <input
                  className={inputCls}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="aws-eu"
                  autoFocus
                  required
                />
              </label>

              <label className="flex flex-col gap-1.5">
                <span className="text-xs font-medium text-muted-foreground">Type</span>
                <select className={inputCls} value={type} onChange={(e) => setType(e.target.value as ProviderType)}>
                  <option value="vsphere">vSphere</option>
                  <option value="proxmox">Proxmox VE</option>
                  <option value="aws">AWS</option>
                  <option value="azure">Azure</option>
                  <option value="gcp">Google Cloud</option>
                </select>
              </label>

              <label className="flex flex-col gap-1.5">
                <span className="text-xs font-medium text-muted-foreground">Secret ref (OpenBao)</span>
                <input
                  className={inputCls}
                  value={secretRef}
                  onChange={(e) => setSecretRef(e.target.value)}
                  placeholder={
                    type === "aws"
                      ? "opord/aws/eu"
                      : type === "azure"
                        ? "opord/azure/dev"
                        : "opord/vsphere/dev"
                  }
                />
                <span className="text-[11px] text-muted-foreground">
                  KV-v2 path in OpenBao where OPORD reads this provider&apos;s credentials. Empty falls back to the
                  process environment.
                </span>
              </label>

              {onprem && (
                <label className="flex flex-col gap-1.5">
                  <span className="text-xs font-medium text-muted-foreground">
                    {type === "vsphere" ? "Server (vCenter FQDN/IP)" : "Server (Proxmox API URL)"}
                  </span>
                  <input
                    className={inputCls}
                    value={server}
                    onChange={(e) => setServer(e.target.value)}
                    placeholder={type === "vsphere" ? "vcenter.example.com" : "https://pve.example.com:8006/api2/json"}
                  />
                </label>
              )}

              {type === "vsphere" && (
                <>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Datacenter</span>
                    <input className={inputCls} value={datacenter} onChange={(e) => setDatacenter(e.target.value)} placeholder="dc-01" />
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Compute cluster</span>
                    <input className={inputCls} value={computeCluster} onChange={(e) => setComputeCluster(e.target.value)} placeholder="cluster-01" />
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Datastore</span>
                    <input className={inputCls} value={datastore} onChange={(e) => setDatastore(e.target.value)} placeholder="datastore-ssd" />
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Network / port group</span>
                    <input className={inputCls} value={network} onChange={(e) => setNetwork(e.target.value)} placeholder="VM Network" />
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">VM folder</span>
                    <input className={inputCls} value={folder} onChange={(e) => setFolder(e.target.value)} placeholder="/dc-01/vm/opord" />
                  </label>
                </>
              )}

              {type === "proxmox" && (
                <>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Node</span>
                    <input className={inputCls} value={node} onChange={(e) => setNode(e.target.value)} placeholder="pve" />
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Datastore</span>
                    <input className={inputCls} value={datastore} onChange={(e) => setDatastore(e.target.value)} placeholder="local-lvm" />
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Network bridge</span>
                    <input className={inputCls} value={network} onChange={(e) => setNetwork(e.target.value)} placeholder="vmbr0" />
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Template VMID</span>
                    <input className={inputCls} value={templateVmid} onChange={(e) => setTemplateVmid(e.target.value)} placeholder="9000" inputMode="numeric" />
                  </label>
                </>
              )}

              {type === "aws" && (
                <>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Region</span>
                    <input className={inputCls} value={region} onChange={(e) => setRegion(e.target.value)} placeholder="eu-central-1" />
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Subnet IDs (EKS)</span>
                    <input className={inputCls} value={subnetIds} onChange={(e) => setSubnetIds(e.target.value)} placeholder="subnet-aaa, subnet-bbb" />
                    <span className="text-[11px] text-muted-foreground">Comma-separated. Required for Kubernetes (EKS).</span>
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Subnet ID (VM/RDS)</span>
                    <input className={inputCls} value={subnetId} onChange={(e) => setSubnetId(e.target.value)} placeholder="subnet-aaa (blank = default VPC)" />
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Security group IDs</span>
                    <input className={inputCls} value={securityGroupIds} onChange={(e) => setSecurityGroupIds(e.target.value)} placeholder="sg-aaa, sg-bbb" />
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">EC2 key pair</span>
                    <input className={inputCls} value={keyName} onChange={(e) => setKeyName(e.target.value)} placeholder="my-keypair (optional)" />
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">OU ID (account factory)</span>
                    <input className={inputCls} value={ouId} onChange={(e) => setOuId(e.target.value)} placeholder="ou-xxxx-xxxxxxxx (blank = root)" />
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Advanced config (JSON)</span>
                    <textarea
                      className="h-24 w-full rounded-lg border border-border bg-card p-2 font-mono text-xs text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                      value={advancedJson}
                      onChange={(e) => setAdvancedJson(e.target.value)}
                      placeholder={'{ "node_instance_type": "t3.medium", "db_instance_class": "db.t3.micro" }'}
                      spellCheck={false}
                    />
                    <span className="text-[11px] text-muted-foreground">Any extra provider keys (merged; typed fields above win).</span>
                  </label>
                </>
              )}

              {type === "azure" && (
                <>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Subscription ID</span>
                    <input
                      className={inputCls}
                      value={subscriptionId}
                      onChange={(e) => setSubscriptionId(e.target.value)}
                      placeholder="00000000-0000-0000-0000-000000000000"
                    />
                    <span className="text-[11px] text-muted-foreground">
                      Azure subscription where resources are created. Not a secret - the service-principal lives in OpenBao.
                    </span>
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Location</span>
                    <input
                      className={inputCls}
                      value={azureLocation}
                      onChange={(e) => setAzureLocation(e.target.value)}
                      placeholder="westeurope"
                    />
                  </label>
                </>
              )}

              {type === "gcp" && (
                <>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Project ID</span>
                    <input
                      className={inputCls}
                      value={projectId}
                      onChange={(e) => setProjectId(e.target.value)}
                      placeholder="my-gcp-project"
                    />
                    <span className="text-[11px] text-muted-foreground">
                      GCP project where resources are created. Not a secret - the service-account JSON key lives in OpenBao (key `credentials`).
                    </span>
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Region</span>
                    <input className={inputCls} value={region} onChange={(e) => setRegion(e.target.value)} placeholder="europe-west1" />
                  </label>
                  <label className="flex flex-col gap-1.5">
                    <span className="text-xs font-medium text-muted-foreground">Zone</span>
                    <input className={inputCls} value={gcpZone} onChange={(e) => setGcpZone(e.target.value)} placeholder="europe-west1-b" />
                  </label>
                </>
              )}

              <div className="flex justify-end gap-2 pt-1">
                <button type="button" onClick={() => setOpen(false)} className={cn(button({ variant: "outline", size: "md" }))}>
                  Cancel
                </button>
                <button type="submit" disabled={busy} className={cn(button({ size: "md" }))}>
                  {busy && <Loader2 className="size-4 animate-spin" />}
                  Register
                </button>
              </div>
            </form>
          </div>,
          document.body,
        )}
    </>
  );
}
