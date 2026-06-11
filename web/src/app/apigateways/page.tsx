import Link from "next/link";
import { Webhook, Plus } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { EmptyState } from "@/components/empty-state";
import { ClusterStatusBadge } from "@/components/status-badge";
import { DestroyButton } from "@/components/destroy-button";
import { DeployTarget } from "@/components/deploy-target";
import { fetchAPIGateways } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

export const metadata = { title: "API gateways" };

export default async function APIGatewaysPage() {
  const gateways = await fetchAPIGateways();

  return (
    <div className="space-y-6">
      <PageHeader
        title="API gateways"
        description="Managed HTTP APIs (API Gateway) fronting a Lambda or upstream HTTP service."
      >
        <Link href="/apigateways/new" className={button({ size: "sm" })}>
          <Plus className="size-4" />
          New API gateway
        </Link>
      </PageHeader>

      {gateways.length === 0 ? (
        <Card>
          <EmptyState
            icon={Webhook}
            title="No API gateways yet"
            description="Create an HTTP API from the catalog or CLI (opord apigateway create)."
            action={{ href: "/apigateways/new", label: "New API gateway" }}
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
                  <th scope="col" className="px-5 py-3 font-medium">Endpoint</th>
                  <th scope="col" className="px-5 py-3 font-medium">API ID</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Created</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {gateways.map((gw) => (
                  <tr key={gw.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <span className="font-medium">{gw.name}</span>
                      <div className="text-xs text-muted-foreground">{gw.environment}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {gw.provider}
                      <DeployTarget account={gw.targetAccount} />
                    </td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">{gw.endpoint ?? "-"}</td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">{gw.apiId ?? "-"}</td>
                    <td className="px-5 py-3">
                      <ClusterStatusBadge status={gw.status} error={gw.lastError} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{timeAgo(gw.createdAt)}</td>
                    <td className="sticky right-0 bg-card px-5 py-3 text-right">
                      <DestroyButton resource="apigateways" name={gw.name} environment={gw.environment} status={gw.status} />
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
