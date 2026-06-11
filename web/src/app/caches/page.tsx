import Link from "next/link";
import { Gauge, Plus } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { EmptyState } from "@/components/empty-state";
import { ClusterStatusBadge } from "@/components/status-badge";
import { DestroyButton } from "@/components/destroy-button";
import { DeployTarget } from "@/components/deploy-target";
import { fetchCaches } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

export const metadata = { title: "Caches" };

export default async function CachesPage() {
  const caches = await fetchCaches();

  return (
    <div className="space-y-6">
      <PageHeader
        title="Caches"
        description="Managed in-memory Redis caches (AWS ElastiCache / Azure Cache for Redis / GCP Memorystore), TLS-only. Access keys are never persisted by OPORD."
      >
        <Link href="/caches/new" className={button({ size: "sm" })}>
          <Plus className="size-4" />
          New cache
        </Link>
      </PageHeader>

      {caches.length === 0 ? (
        <Card>
          <EmptyState
            icon={Gauge}
            title="No caches yet"
            description="Provision a Redis cache from the catalog or CLI (opord cache create)."
            action={{ href: "/caches/new", label: "New cache" }}
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
                  <th scope="col" className="px-5 py-3 font-medium">Cache</th>
                  <th scope="col" className="px-5 py-3 font-medium">Endpoint</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Created</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {caches.map((cache) => (
                  <tr key={cache.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <span className="font-medium">{cache.name}</span>
                      <div className="text-xs text-muted-foreground">{cache.environment}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {cache.provider}
                      <DeployTarget account={cache.targetAccount} />
                    </td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">{cache.cacheName}</td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">{cache.primaryEndpoint || "-"}</td>
                    <td className="px-5 py-3">
                      <ClusterStatusBadge status={cache.status} error={cache.lastError} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{timeAgo(cache.createdAt)}</td>
                    <td className="sticky right-0 bg-card px-5 py-3 text-right">
                      <DestroyButton resource="caches" name={cache.name} environment={cache.environment} status={cache.status} />
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
