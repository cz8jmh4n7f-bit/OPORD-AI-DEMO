import Link from "next/link";
import { Globe, Plus } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { EmptyState } from "@/components/empty-state";
import { ClusterStatusBadge } from "@/components/status-badge";
import { DestroyButton } from "@/components/destroy-button";
import { DeployTarget } from "@/components/deploy-target";
import { fetchDNS } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

export const metadata = { title: "DNS zones" };

export default async function DNSPage() {
  const zones = await fetchDNS();

  return (
    <div className="space-y-6">
      <PageHeader
        title="DNS zones"
        description="Managed Route 53 hosted zones (public or private) for the expose layer."
      >
        <Link href="/dns/new" className={button({ size: "sm" })}>
          <Plus className="size-4" />
          New zone
        </Link>
      </PageHeader>

      {zones.length === 0 ? (
        <Card>
          <EmptyState
            icon={Globe}
            title="No DNS zones yet"
            description="Create a public or private hosted zone from the catalog or CLI (opord dns create)."
            action={{ href: "/dns/new", label: "New zone" }}
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
                  <th scope="col" className="px-5 py-3 font-medium">Zone</th>
                  <th scope="col" className="px-5 py-3 font-medium">Name servers</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Created</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {zones.map((zone) => (
                  <tr key={zone.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <span className="font-medium">{zone.name}</span>
                      <div className="text-xs text-muted-foreground">{zone.environment}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {zone.provider}
                      <DeployTarget account={zone.targetAccount} />
                    </td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">{zone.zoneName ?? "-"}</td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">
                      {zone.nameServers && zone.nameServers.length > 0 ? zone.nameServers.join(", ") : "-"}
                    </td>
                    <td className="px-5 py-3">
                      <ClusterStatusBadge status={zone.status} error={zone.lastError} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{timeAgo(zone.createdAt)}</td>
                    <td className="sticky right-0 bg-card px-5 py-3 text-right">
                      <DestroyButton resource="dns" name={zone.name} environment={zone.environment} status={zone.status} />
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
