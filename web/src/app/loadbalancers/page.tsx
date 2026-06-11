import Link from "next/link";
import { Network, Plus } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { EmptyState } from "@/components/empty-state";
import { ClusterStatusBadge } from "@/components/status-badge";
import { DestroyButton } from "@/components/destroy-button";
import { DeployTarget } from "@/components/deploy-target";
import { fetchLoadBalancers } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

export const metadata = { title: "Load balancers" };

export default async function LoadBalancersPage() {
  const lbs = await fetchLoadBalancers();

  return (
    <div className="space-y-6">
      <PageHeader
        title="Load balancers"
        description="Managed application load balancers (ALB) that front your compute targets."
      >
        <Link href="/loadbalancers/new" className={button({ size: "sm" })}>
          <Plus className="size-4" />
          New load balancer
        </Link>
      </PageHeader>

      {lbs.length === 0 ? (
        <Card>
          <EmptyState
            icon={Network}
            title="No load balancers yet"
            description="Create an application load balancer from the catalog or CLI (opord loadbalancer create)."
            action={{ href: "/loadbalancers/new", label: "New load balancer" }}
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
                  <th scope="col" className="px-5 py-3 font-medium">DNS name</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Created</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {lbs.map((lb) => (
                  <tr key={lb.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <span className="font-medium">{lb.name}</span>
                      <div className="text-xs text-muted-foreground">{lb.environment}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {lb.provider}
                      <DeployTarget account={lb.targetAccount} />
                    </td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">{lb.dnsName ?? "-"}</td>
                    <td className="px-5 py-3">
                      <ClusterStatusBadge status={lb.status} error={lb.lastError} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{timeAgo(lb.createdAt)}</td>
                    <td className="sticky right-0 bg-card px-5 py-3 text-right">
                      <DestroyButton resource="loadbalancers" name={lb.name} environment={lb.environment} status={lb.status} />
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
