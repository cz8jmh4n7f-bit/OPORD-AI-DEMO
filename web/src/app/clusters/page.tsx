import Link from "next/link";
import { Boxes, Plus } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { EmptyState } from "@/components/empty-state";
import { ClusterStatusBadge } from "@/components/status-badge";
import { DestroyButton } from "@/components/destroy-button";
import { fetchClusters } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

export const metadata = { title: "Clusters" };

export default async function ClustersPage() {
  const clusters = await fetchClusters();

  return (
    <div className="space-y-6">
      <PageHeader title="Clusters" description="Kubernetes clusters managed by OPORD.">
        <Link href="/clusters/new" className={button({ size: "sm" })}>
          <Plus className="size-4" />
          New cluster
        </Link>
      </PageHeader>

      {clusters.length === 0 ? (
        <Card>
          <EmptyState
            icon={Boxes}
            title="No clusters yet"
            description="Provision a Kubernetes cluster on any connected provider - vSphere, Proxmox, or AWS."
            action={{ href: "/clusters/new", label: "New cluster" }}
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
                  <th scope="col" className="px-5 py-3 font-medium">Nodes</th>
                  <th scope="col" className="px-5 py-3 font-medium">K8s</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Updated</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {clusters.map((c) => (
                  <tr key={c.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <Link
                        href={`/clusters/${c.name}`}
                        className="font-medium text-foreground hover:text-primary"
                      >
                        {c.name}
                      </Link>
                      <div className="text-xs text-muted-foreground">{c.environment}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{c.provider}</td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {c.controlPlanes} CP / {c.workers} W
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{c.kubernetesVersion}</td>
                    <td className="px-5 py-3">
                      <ClusterStatusBadge status={c.status} error={c.lastError} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{timeAgo(c.updatedAt)}</td>
                    <td className="sticky right-0 bg-card px-5 py-3 text-right">
                      <DestroyButton resource="clusters" name={c.name} environment={c.environment} status={c.status} />
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
