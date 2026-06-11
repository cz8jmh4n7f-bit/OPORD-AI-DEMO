import Link from "next/link";
import { Database, Plus } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { EmptyState } from "@/components/empty-state";
import { ClusterStatusBadge } from "@/components/status-badge";
import { DestroyButton } from "@/components/destroy-button";
import { DeployTarget } from "@/components/deploy-target";
import { fetchDatabases } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

export const metadata = { title: "Databases" };

export default async function DatabasesPage() {
  const databases = await fetchDatabases();

  return (
    <div className="space-y-6">
      <PageHeader
        title="Databases"
        description="Managed databases (e.g. AWS RDS, GCP Cloud SQL, Azure PostgreSQL). OPORD never holds the master password."
      >
        <Link href="/databases/new" className={button({ size: "sm" })}>
          <Plus className="size-4" />
          New database
        </Link>
      </PageHeader>

      {databases.length === 0 ? (
        <Card>
          <EmptyState
            icon={Database}
            title="No databases yet"
            description="Provision a managed database (RDS) from the catalog, an environment, or the CLI. OPORD never holds the master password - RDS stores it in Secrets Manager."
            action={{ href: "/databases/new", label: "New database" }}
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
                  <th scope="col" className="px-5 py-3 font-medium">Engine</th>
                  <th scope="col" className="px-5 py-3 font-medium">Class</th>
                  <th scope="col" className="px-5 py-3 font-medium">Endpoint</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Created</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {databases.map((db) => (
                  <tr key={db.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <span className="font-medium">{db.name}</span>
                      <div className="text-xs text-muted-foreground">{db.environment}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {db.provider}
                      <DeployTarget account={db.targetAccount} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {db.engine}
                      {db.version ? ` ${db.version}` : ""}
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {db.instanceClass ?? "-"}
                      {` · ${db.storageGb} GB`}
                    </td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">
                      {db.endpoint ? `${db.endpoint}${db.port ? `:${db.port}` : ""}` : "-"}
                    </td>
                    <td className="px-5 py-3">
                      <ClusterStatusBadge status={db.status} error={db.lastError} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{timeAgo(db.createdAt)}</td>
                    <td className="sticky right-0 bg-card px-5 py-3 text-right">
                      <DestroyButton resource="databases" name={db.name} environment={db.environment} status={db.status} />
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
