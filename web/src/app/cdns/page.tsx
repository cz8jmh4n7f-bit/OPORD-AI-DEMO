import Link from "next/link";
import { Globe2, Plus } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { EmptyState } from "@/components/empty-state";
import { ClusterStatusBadge } from "@/components/status-badge";
import { DestroyButton } from "@/components/destroy-button";
import { DeployTarget } from "@/components/deploy-target";
import { fetchCDNs } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

export const metadata = { title: "CDNs" };

export default async function CDNsPage() {
  const cdns = await fetchCDNs();

  return (
    <div className="space-y-6">
      <PageHeader
        title="CDNs"
        description="Managed content delivery (CloudFront) distributions fronting your origins."
      >
        <Link href="/cdns/new" className={button({ size: "sm" })}>
          <Plus className="size-4" />
          New CDN
        </Link>
      </PageHeader>

      {cdns.length === 0 ? (
        <Card>
          <EmptyState
            icon={Globe2}
            title="No CDNs yet"
            description="Create a CloudFront distribution from the catalog or CLI (opord cdn create)."
            action={{ href: "/cdns/new", label: "New CDN" }}
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
                  <th scope="col" className="px-5 py-3 font-medium">Domain</th>
                  <th scope="col" className="px-5 py-3 font-medium">Distribution</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Created</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {cdns.map((cdn) => (
                  <tr key={cdn.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <span className="font-medium">{cdn.name}</span>
                      <div className="text-xs text-muted-foreground">{cdn.environment}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {cdn.provider}
                      <DeployTarget account={cdn.targetAccount} />
                    </td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">{cdn.domainName ?? "-"}</td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">{cdn.distributionId ?? "-"}</td>
                    <td className="px-5 py-3">
                      <ClusterStatusBadge status={cdn.status} error={cdn.lastError} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{timeAgo(cdn.createdAt)}</td>
                    <td className="sticky right-0 bg-card px-5 py-3 text-right">
                      <DestroyButton resource="cdns" name={cdn.name} environment={cdn.environment} status={cdn.status} />
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
