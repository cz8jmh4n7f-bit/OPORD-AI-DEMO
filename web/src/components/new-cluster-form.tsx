"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Check, Loader2, TriangleAlert, Info } from "lucide-react";
import type { Provider } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { authHeaders } from "@/lib/client-auth";
import { DeployIntoField } from "@/components/deploy-into-field";

const API = "/bff";
const inputCls =
  "h-9 w-full rounded-lg border border-input bg-card px-3 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";
const textareaCls =
  "w-full rounded-lg border border-input bg-card px-3 py-2 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring resize-y";

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

export function NewClusterForm({
  providers,
  initialProvider,
}: {
  providers: Provider[];
  initialProvider?: string;
}) {
  const router = useRouter();
  const preset =
    initialProvider && providers.some((p) => p.name === initialProvider)
      ? initialProvider
      : undefined;

  // Identity
  const [name, setName] = useState("");
  const [environment, setEnvironment] = useState("dev");
  const [provider, setProvider] = useState(
    preset ??
      providers.find((p) => p.type === "vsphere")?.name ??
      providers[0]?.name ??
      ""
  );

  // Kubernetes
  const [kubernetesVersion, setKubernetesVersion] = useState("");
  const [versionCustom, setVersionCustom] = useState(false);
  const [clusterVersions, setClusterVersions] = useState<string[]>([]);
  const [cni, setCni] = useState<"cilium" | "calico" | "flannel">("cilium");
  const [template, setTemplate] = useState("debian-12");

  // Node counts
  const [cpCount, setCpCount] = useState(1);
  const [workerCount, setWorkerCount] = useState(2);

  // Shared node specs (applied to both groups) - kubeadm path only.
  const [cpu, setCpu] = useState(2);
  const [memoryMb, setMemoryMb] = useState(4096);
  const [diskGb, setDiskGb] = useState(40);

  // Managed-cluster node size: a machine type (GCP) / instance type (AWS) / VM
  // size (Azure). "" = the provider config's default; "__custom__" reveals a text box.
  const [machineType, setMachineType] = useState("");
  const [customMachine, setCustomMachine] = useState("");

  // On-prem networking
  const [cpIpStart, setCpIpStart] = useState("");
  const [workerIpStart, setWorkerIpStart] = useState("");
  const [gateway, setGateway] = useState("");
  const [netmask, setNetmask] = useState("24");
  const [dnsServers, setDnsServers] = useState("");
  const [cpEndpoint, setCpEndpoint] = useState("");

  // SSH
  const [sshUser, setSshUser] = useState("opord");
  const [sshPublicKey, setSshPublicKey] = useState("");

  // Deploy-into (ADR-0013)
  const [targetAccount, setTargetAccount] = useState("");

  // UI state
  const [submitting, setSubmitting] = useState(false);
  const [dryRunning, setDryRunning] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);

  const selectedType = providers.find((p) => p.name === provider)?.type;
  const isOnPrem = selectedType === "vsphere" || selectedType === "proxmox";
  const isAws = selectedType === "aws";
  const isAzure = selectedType === "azure";
  const isGCP = selectedType === "gcp";
  const isManaged = isAws || isAzure || isGCP;
  // Common node sizes per managed provider (+ a "Custom…" escape hatch).
  const machineOptions: Record<string, string[]> = {
    gcp: ["e2-small", "e2-medium", "e2-standard-2", "e2-standard-4", "n2-standard-2"],
    aws: ["t3.small", "t3.medium", "t3.large", "t3.xlarge", "m5.large"],
    azure: ["Standard_B2s", "Standard_B2ms", "Standard_D2s_v5", "Standard_D4s_v5"],
  };
  const mOpts = machineOptions[selectedType ?? ""] ?? [];
  const effectiveMachine = machineType === "__custom__" ? customMachine.trim() : machineType;
  // LIVE versions the managed service (GKE/AKS/EKS) offers RIGHT NOW, fetched per
  // provider (GKE serverConfig). A hardcoded list goes stale fast - GKE already
  // dropped 1.31 for 1.35. Empty (fetch failed / keyless ADC / cloud not yet
  // implemented) to the dropdown shows just "(provider default)" + "Custom…".
  useEffect(() => {
    if (!isManaged || !provider) {
      return;
    }
    let cancelled = false;
    void (async () => {
      try {
        const res = await fetch(
          `${API}/api/v1/providers/${encodeURIComponent(provider)}/cluster-versions`,
          { headers: authHeaders() },
        );
        const data: unknown = res.ok ? await res.json() : [];
        if (!cancelled) setClusterVersions(Array.isArray(data) ? (data as string[]) : []);
      } catch {
        if (!cancelled) setClusterVersions([]);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [isManaged, provider]);

  function buildSpec() {
    const dns = dnsServers
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);

    return {
      kubernetes_version: kubernetesVersion,
      cni,
      machine_type: isManaged ? effectiveMachine : "",
      template,
      control_plane: {
        count: cpCount,
        name_prefix: `${name}-cp`,
        ip_start: isOnPrem ? cpIpStart : "",
        specs: { cpu, memory_mb: memoryMb, disk_gb: diskGb },
      },
      workers: {
        count: workerCount,
        name_prefix: `${name}-worker`,
        ip_start: isOnPrem ? workerIpStart : "",
        specs: { cpu, memory_mb: memoryMb, disk_gb: diskGb },
      },
      networking: {
        netmask: isOnPrem ? netmask : "",
        gateway: isOnPrem ? gateway : "",
        dns_servers: isOnPrem ? dns : [],
        control_plane_endpoint: isOnPrem ? cpEndpoint : "",
      },
      ssh_user: sshUser,
      ssh_public_key: sshPublicKey,
      ...(((isAws || isAzure || isGCP) && targetAccount.trim())
        ? { target_account: targetAccount.trim() }
        : {}),
    };
  }

  async function doPost(dryRun: boolean) {
    setError(null);
    setOk(null);
    if (dryRun) setDryRunning(true);
    else setSubmitting(true);

    try {
      const body: Record<string, unknown> = {
        name,
        environment,
        provider,
        spec: buildSpec(),
      };
      if (dryRun) body.dryRun = true;

      const res = await fetch(`${API}/api/v1/clusters`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify(body),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `Request failed (${res.status})`);
        return;
      }
      if (dryRun) {
        setOk(`Dry-run OK. ${data.summary ?? "Spec validated."}`);
      } else {
        setOk(`Cluster "${data.name}" created - status ${data.status}.`);
        router.refresh();
        setTimeout(() => router.push("/clusters"), 900);
      }
    } catch (err) {
      setError(String(err));
    } finally {
      setSubmitting(false);
      setDryRunning(false);
    }
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    void doPost(false);
  }

  function handleDryRun(e: React.MouseEvent) {
    e.preventDefault();
    void doPost(true);
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
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

      {/* ── Identity ── */}
      <Card>
        <CardHeader>
          <CardTitle>Cluster</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Name">
            <input
              className={inputCls}
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="k8s-dev"
              required
            />
          </Field>
          <Field label="Environment">
            <select
              className={inputCls}
              value={environment}
              onChange={(e) => setEnvironment(e.target.value)}
            >
              <option value="dev">dev</option>
              <option value="staging">staging</option>
              <option value="production">production</option>
            </select>
          </Field>
          <Field
            label="Provider"
            hint={
              isOnPrem
                ? "On-prem - kubeadm bootstrap via Ansible."
                : isAws
                ? "AWS EKS - managed control plane."
                : isAzure
                ? "Azure AKS - managed control plane."
                : isGCP
                ? "GCP GKE - managed control plane."
                : undefined
            }
          >
            <select
              className={inputCls}
              value={provider}
              onChange={(e) => {
                setProvider(e.target.value);
                setTargetAccount("");
              }}
              required
            >
              {providers.length === 0 && (
                <option value="">no providers registered</option>
              )}
              {providers.map((p) => (
                <option key={p.id} value={p.name}>
                  {p.name} ({p.type})
                </option>
              ))}
            </select>
          </Field>
          <DeployIntoField
            provider={provider}
            providerType={selectedType}
            value={targetAccount}
            onChange={setTargetAccount}
          />
        </CardContent>
      </Card>

      {/* ── Kubernetes ── */}
      <Card>
        <CardHeader>
          <CardTitle>Kubernetes</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          {isManaged ? (
            <Field
              label="Kubernetes version"
              hint={
                clusterVersions.length > 0
                  ? "Live versions the managed service offers right now (it resolves the latest patch). Default = its latest stable."
                  : "Default = the service's latest stable. (Live version list unavailable - pick Custom… for a specific version.)"
              }
            >
              <select
                className={inputCls}
                value={versionCustom ? "__custom__" : kubernetesVersion}
                onChange={(e) => {
                  if (e.target.value === "__custom__") {
                    setVersionCustom(true);
                  } else {
                    setVersionCustom(false);
                    setKubernetesVersion(e.target.value);
                  }
                }}
              >
                <option value="">(provider default)</option>
                {clusterVersions.map((v) => (
                  <option key={v} value={v}>
                    {v}
                  </option>
                ))}
                <option value="__custom__">Custom…</option>
              </select>
              {versionCustom && (
                <input
                  className={`${inputCls} mt-2`}
                  value={kubernetesVersion}
                  onChange={(e) => setKubernetesVersion(e.target.value)}
                  placeholder="e.g. 1.30.5"
                />
              )}
            </Field>
          ) : (
            <Field label="Kubernetes version" hint="Leave empty for the provider's default (kubeadm).">
              <input
                className={inputCls}
                value={kubernetesVersion}
                onChange={(e) => setKubernetesVersion(e.target.value)}
                placeholder="(provider default)"
              />
            </Field>
          )}
          <Field label="CNI">
            <select
              className={inputCls}
              value={cni}
              onChange={(e) =>
                setCni(e.target.value as "cilium" | "calico" | "flannel")
              }
            >
              <option value="cilium">cilium</option>
              <option value="calico">calico</option>
              <option value="flannel">flannel</option>
            </select>
          </Field>
          {isOnPrem && (
            <Field
              label="Node image template"
              hint="VM template name in vSphere/Proxmox (e.g. debian-12)."
            >
              <input
                className={inputCls}
                value={template}
                onChange={(e) => setTemplate(e.target.value)}
                placeholder="debian-12"
              />
            </Field>
          )}
          {!isOnPrem && (
            <div className="sm:col-span-2 flex items-start gap-2 rounded-xl border border-info/30 bg-info/10 p-3 text-sm text-info">
              <Info className="mt-0.5 size-4 shrink-0" />
              <span>
                Managed control plane - OPORD skips Ansible. Kubeconfig is
                fetched via the cloud provider CLI after provisioning.
              </span>
            </div>
          )}
        </CardContent>
      </Card>

      {/* ── Topology ── */}
      <Card>
        <CardHeader>
          <CardTitle>Topology</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          {isManaged ? (
            <>
              {/* Managed clusters: the provider owns the control plane (no CP-node
                  count) and sizes nodes by a machine type, not raw CPU/Memory. */}
              <Field label="Machine type" hint="Worker node size. Default = the provider config's default.">
                <select className={inputCls} value={machineType} onChange={(e) => setMachineType(e.target.value)}>
                  <option value="">(provider default)</option>
                  {mOpts.map((m) => (
                    <option key={m} value={m}>
                      {m}
                    </option>
                  ))}
                  <option value="__custom__">Custom…</option>
                </select>
              </Field>
              <Field label="Worker nodes" hint="Size of the managed node pool.">
                <input
                  type="number"
                  min={1}
                  className={inputCls}
                  value={workerCount}
                  onChange={(e) => setWorkerCount(Number(e.target.value))}
                />
              </Field>
              <Field label="Disk (GB per node)">
                <input
                  type="number"
                  min={10}
                  className={inputCls}
                  value={diskGb}
                  onChange={(e) => setDiskGb(Number(e.target.value))}
                />
              </Field>
              {machineType === "__custom__" && (
                <div className="sm:col-span-3">
                  <Field label="Custom machine type" hint={`A provider-specific size, e.g. ${mOpts[0] ?? "…"}.`}>
                    <input
                      className={inputCls}
                      value={customMachine}
                      onChange={(e) => setCustomMachine(e.target.value)}
                      placeholder={mOpts[0] ?? ""}
                    />
                  </Field>
                </div>
              )}
            </>
          ) : (
            <>
              <Field label="Control plane nodes">
                <input
                  type="number"
                  min={1}
                  className={inputCls}
                  value={cpCount}
                  onChange={(e) => setCpCount(Number(e.target.value))}
                />
              </Field>
              <Field label="Worker nodes">
                <input
                  type="number"
                  min={1}
                  className={inputCls}
                  value={workerCount}
                  onChange={(e) => setWorkerCount(Number(e.target.value))}
                />
              </Field>
              {/* spacer on large screens */}
              <div className="hidden sm:block" />
              <Field label="CPU (vCPU per node)">
                <input
                  type="number"
                  min={1}
                  className={inputCls}
                  value={cpu}
                  onChange={(e) => setCpu(Number(e.target.value))}
                />
              </Field>
              <Field label="Memory (MB per node)">
                <input
                  type="number"
                  min={512}
                  step={512}
                  className={inputCls}
                  value={memoryMb}
                  onChange={(e) => setMemoryMb(Number(e.target.value))}
                />
              </Field>
              <Field label="Disk (GB per node)">
                <input
                  type="number"
                  min={10}
                  className={inputCls}
                  value={diskGb}
                  onChange={(e) => setDiskGb(Number(e.target.value))}
                />
              </Field>
            </>
          )}
        </CardContent>
      </Card>

      {/* ── Networking (on-prem only) ── */}
      {isOnPrem && (
        <Card>
          <CardHeader>
            <CardTitle>Networking</CardTitle>
          </CardHeader>
          <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Field
              label="Control plane IP start"
              hint="First IP in the static range for control plane VMs."
            >
              <input
                className={inputCls}
                value={cpIpStart}
                onChange={(e) => setCpIpStart(e.target.value)}
                placeholder="10.0.0.80"
              />
            </Field>
            <Field
              label="Workers IP start"
              hint="First IP in the static range for worker VMs."
            >
              <input
                className={inputCls}
                value={workerIpStart}
                onChange={(e) => setWorkerIpStart(e.target.value)}
                placeholder="10.0.0.90"
              />
            </Field>
            <Field label="Gateway">
              <input
                className={inputCls}
                value={gateway}
                onChange={(e) => setGateway(e.target.value)}
                placeholder="10.0.0.1"
              />
            </Field>
            <Field label="Netmask (prefix length)" hint="e.g. 24 for /24">
              <input
                className={inputCls}
                value={netmask}
                onChange={(e) => setNetmask(e.target.value)}
                placeholder="24"
              />
            </Field>
            <Field
              label="DNS servers"
              hint="Comma-separated list, e.g. 192.168.2.3, 8.8.8.8"
            >
              <input
                className={inputCls}
                value={dnsServers}
                onChange={(e) => setDnsServers(e.target.value)}
                placeholder="192.168.2.3"
              />
            </Field>
            <Field
              label="Control plane endpoint"
              hint="VIP or first CP IP - the address kubeconfig will point to."
            >
              <input
                className={inputCls}
                value={cpEndpoint}
                onChange={(e) => setCpEndpoint(e.target.value)}
                placeholder="10.0.0.80"
              />
            </Field>
          </CardContent>
        </Card>
      )}

      {/* ── SSH (kubeadm only) ── OPORD creates + Ansible-bootstraps the node VMs
          only for on-prem (vSphere/Proxmox), so an SSH user/key only makes sense
          there. Managed control planes (EKS/AKS/GKE) own their node pool and have
          no SSH access, so this section is hidden - it was previously ignored. */}
      {!isManaged && (
        <Card>
          <CardHeader>
            <CardTitle>SSH</CardTitle>
          </CardHeader>
          <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Field label="SSH user">
              <input
                className={inputCls}
                value={sshUser}
                onChange={(e) => setSshUser(e.target.value)}
                placeholder="opord"
              />
            </Field>
            <div className="sm:col-span-2">
              <Field
                label="SSH public key (optional)"
                hint="Injected into node VMs at provision time. Leave blank to rely on the provider template's default key."
              >
                <textarea
                  className={textareaCls}
                  rows={3}
                  value={sshPublicKey}
                  onChange={(e) => setSshPublicKey(e.target.value)}
                  placeholder="ssh-ed25519 AAAA..."
                />
              </Field>
            </div>
          </CardContent>
        </Card>
      )}

      <div className="flex flex-wrap items-center gap-3">
        <button
          type="submit"
          disabled={submitting || dryRunning}
          className={cn(button({ size: "md" }), (submitting || dryRunning) && "opacity-70")}
        >
          {submitting && <Loader2 className="size-4 animate-spin" />}
          Create cluster
        </button>
        <button
          type="button"
          onClick={handleDryRun}
          disabled={submitting || dryRunning}
          className={cn(
            button({ variant: "outline", size: "md" }),
            (submitting || dryRunning) && "opacity-70"
          )}
        >
          {dryRunning && <Loader2 className="size-4 animate-spin" />}
          Validate (dry-run)
        </button>
        <Link href="/clusters" className={button({ variant: "outline", size: "md" })}>
          Cancel
        </Link>
      </div>
    </form>
  );
}
