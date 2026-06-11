import Link from "next/link";
import { notFound } from "next/navigation";
import { ChevronLeft } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { DestroyButton } from "@/components/destroy-button";
import { ScaleClusterButton } from "@/components/scale-cluster-button";
import { ClusterStatusBadge, JobStatusBadge } from "@/components/status-badge";
import { fetchCluster } from "@/lib/api";
import type { ClusterNode } from "@/lib/types";
import { cn, formatDate } from "@/lib/utils";

export const metadata = { title: "Clusters" };

const nodeBadge: Record<ClusterNode["status"], "default" | "info" | "success" | "danger"> = {
  pending: "default",
  provisioned: "info",
  ready: "success",
  failed: "danger",
};

function Info({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <dt className="text-xs uppercase tracking-wide text-muted-foreground">{label}</dt>
      <dd className={cn("mt-1 text-sm font-medium", mono && "font-mono")}>{value}</dd>
    </div>
  );
}

export default async function ClusterDetailPage({
  params,
}: {
  params: Promise<{ name: string }>;
}) {
  const { name } = await params;
  const cluster = await fetchCluster(decodeURIComponent(name));
  if (!cluster) notFound();

  const nodes = cluster.nodes ?? [];
  const jobs = cluster.jobs;

  return (
    <div className="space-y-6">
      <Link
        href="/clusters"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ChevronLeft className="size-4" />
        Clusters
      </Link>

      <PageHeader title={cluster.name} description={`${cluster.environment} · ${cluster.provider}`}>
        <ClusterStatusBadge status={cluster.status} error={cluster.lastError} />
        <ScaleClusterButton
          name={cluster.name}
          environment={cluster.environment}
          workers={cluster.workers}
          status={cluster.status}
        />
        <DestroyButton
          resource="clusters"
          name={cluster.name}
          environment={cluster.environment}
          status={cluster.status}
        />
      </PageHeader>

      <Card>
        <dl className="grid grid-cols-2 gap-5 p-5 sm:grid-cols-3 lg:grid-cols-6">
          <Info label="Provider" value={cluster.provider} />
          <Info label="Endpoint" value={cluster.endpoint || "-"} mono />
          <Info label="Kubernetes" value={cluster.kubernetesVersion ? `v${cluster.kubernetesVersion}` : "-"} />
          <Info label="CNI" value={cluster.cni || "-"} />
          <Info label="Created" value={formatDate(cluster.createdAt)} />
          <Info label="Updated" value={formatDate(cluster.updatedAt)} />
        </dl>
        <dl className="border-t border-border px-5 py-3">
          <dt className="text-xs uppercase tracking-wide text-muted-foreground">Kubeconfig</dt>
          <dd className="mt-1 break-all font-mono text-xs text-muted-foreground">
            {cluster.kubeconfigRef || "- available after the cluster reaches “ready”"}
          </dd>
        </dl>
      </Card>

      <div className="grid gap-6 lg:grid-cols-3">
        <Card className="lg:col-span-2 overflow-hidden p-0">
          <CardHeader className="p-5 pb-3">
            <CardTitle>
              {cluster.managed ? "Node pool" : "Nodes"}{" "}
              <span className="text-muted-foreground">({nodes.length})</span>
            </CardTitle>
          </CardHeader>
          {nodes.length === 0 ? (
            <p className="px-5 pb-5 text-sm text-muted-foreground">
              {cluster.managed
                ? `Managed control plane - ${cluster.provider} owns the node pool, so OPORD does not track individual nodes. List them with the kubeconfig command above: kubectl get nodes.`
                : "No nodes yet - provisioning has not run."}
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-y border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
                    <th scope="col" className="px-5 py-2.5 font-medium">Name</th>
                    <th scope="col" className="px-5 py-2.5 font-medium">Role</th>
                    <th scope="col" className="px-5 py-2.5 font-medium">IP</th>
                    <th scope="col" className="px-5 py-2.5 font-medium">Status</th>
                  </tr>
                </thead>
                <tbody>
                  {nodes.map((n) => (
                    <tr key={n.name} className="border-b border-border last:border-0">
                      <td className="px-5 py-2.5 font-medium">{n.name}</td>
                      <td className="px-5 py-2.5">
                        <Badge variant={n.role === "control_plane" ? "primary" : "default"}>
                          {n.role === "control_plane" ? "control-plane" : "worker"}
                        </Badge>
                      </td>
                      <td className="px-5 py-2.5 font-mono text-xs text-muted-foreground">{n.ip || "-"}</td>
                      <td className="px-5 py-2.5">
                        <Badge variant={nodeBadge[n.status]}>{n.status}</Badge>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Jobs</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3.5">
            {jobs.length === 0 && <p className="text-sm text-muted-foreground">No jobs yet.</p>}
            {jobs.map((j) => {
              const when = j.startedAt ?? j.createdAt ?? null;
              return (
                <div key={j.id} className="flex items-center justify-between gap-3">
                  <div className="min-w-0">
                    <div className="text-sm font-medium">{j.operation}</div>
                    <div className="text-xs text-muted-foreground">{when ? formatDate(when) : "queued"}</div>
                  </div>
                  <JobStatusBadge status={j.status} />
                </div>
              );
            })}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
