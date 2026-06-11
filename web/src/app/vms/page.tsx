import Link from "next/link";
import { Cpu, Plus } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { EmptyState } from "@/components/empty-state";
import { ClusterStatusBadge } from "@/components/status-badge";
import { DestroyButton } from "@/components/destroy-button";
import { ScaleVMButton } from "@/components/scale-vm-button";
import { DeployTarget } from "@/components/deploy-target";
import { fetchVMs } from "@/lib/api";
import type { VM } from "@/lib/types";
import { timeAgo } from "@/lib/utils";

export const metadata = { title: "Virtual machines" };

// vmIp shows the primary assigned IP (public preferred, else private), with a
// +N badge for extra instances. Terminal VMs have no live IP, so show a dash.
function vmIp(vm: VM): string {
  if (vm.status === "destroyed" || vm.status === "destroying") return "-";
  const ips = (vm.publicIps?.length ? vm.publicIps : vm.privateIps) ?? [];
  if (ips.length === 0) return "-";
  return ips.length > 1 ? `${ips[0]} +${ips.length - 1}` : ips[0];
}

export default async function VMsPage() {
  const vms = await fetchVMs();

  return (
    <div className="space-y-6">
      <PageHeader title="Virtual machines" description="Self-service VMs provisioned by OPORD.">
        <Link href="/vms/new" className={button({ size: "sm" })}>
          <Plus className="size-4" />
          New VM
        </Link>
      </PageHeader>

      {vms.length === 0 ? (
        <Card>
          <EmptyState
            icon={Cpu}
            title="No virtual machines yet"
            description="Spin up a VM on vSphere, Proxmox, or AWS - sized by preset or instance type, with optional auto-destroy."
            action={{ href: "/vms/new", label: "New VM" }}
          />
        </Card>
      ) : (
        <Card className="overflow-hidden p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
                  <th scope="col" className="px-5 py-3 font-medium">Name</th>
                  <th scope="col" className="px-5 py-3 font-medium">Provider</th>
                  <th scope="col" className="px-5 py-3 font-medium">Compute</th>
                  <th scope="col" className="px-5 py-3 font-medium">IP</th>
                  <th scope="col" className="px-5 py-3 font-medium">Count</th>
                  <th scope="col" className="px-5 py-3 font-medium">TTL</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Created</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {vms.map((vm) => (
                  <tr key={vm.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <span className="font-medium">{vm.name}</span>
                      <div className="text-xs text-muted-foreground">{vm.environment}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {vm.provider}
                      <DeployTarget account={vm.targetAccount} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {vm.instanceType
                        ? vm.instanceType
                        : `${vm.cpu} vCPU · ${Math.round(vm.memoryMb / 1024)} GB`}
                      {` · ${vm.diskGb} GB`}
                    </td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">{vmIp(vm)}</td>
                    <td className="px-5 py-3 text-muted-foreground">{vm.count}</td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {vm.ttlHours && vm.ttlHours > 0 ? `${vm.ttlHours}h` : "-"}
                    </td>
                    <td className="px-5 py-3">
                      <ClusterStatusBadge status={vm.status} error={vm.lastError} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{timeAgo(vm.createdAt)}</td>
                    <td className="sticky right-0 bg-card px-5 py-3">
                      <div className="flex items-center justify-end gap-2">
                        <ScaleVMButton name={vm.name} environment={vm.environment} count={vm.count} status={vm.status} />
                        <DestroyButton resource="vms" name={vm.name} environment={vm.environment} status={vm.status} />
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}
    </div>
  );
}
