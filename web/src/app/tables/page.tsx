import Link from "next/link";
import { Table2, Plus } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { EmptyState } from "@/components/empty-state";
import { ClusterStatusBadge } from "@/components/status-badge";
import { DestroyButton } from "@/components/destroy-button";
import { DeployTarget } from "@/components/deploy-target";
import { fetchTables } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

export const metadata = { title: "Tables" };

export default async function TablesPage() {
  const tables = await fetchTables();

  return (
    <div className="space-y-6">
      <PageHeader
        title="Tables"
        description="Managed NoSQL tables (AWS DynamoDB / GCP Firestore / Azure Cosmos) - a partition key, optional sort key, on-demand or provisioned capacity."
      >
        <Link href="/tables/new" className={button({ size: "sm" })}>
          <Plus className="size-4" />
          New table
        </Link>
      </PageHeader>

      {tables.length === 0 ? (
        <Card>
          <EmptyState
            icon={Table2}
            title="No tables yet"
            description="Provision a managed DynamoDB table - a partition key, optional sort key, on-demand or provisioned capacity."
            action={{ href: "/tables/new", label: "New table" }}
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
                  <th scope="col" className="px-5 py-3 font-medium">Keys</th>
                  <th scope="col" className="px-5 py-3 font-medium">Billing</th>
                  <th scope="col" className="px-5 py-3 font-medium">ARN</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Created</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {tables.map((t) => (
                  <tr key={t.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <span className="font-medium">{t.name}</span>
                      <div className="text-xs text-muted-foreground">{t.environment}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {t.provider}
                      <DeployTarget account={t.targetAccount} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {t.hashKey}
                      {t.rangeKey ? ` / ${t.rangeKey}` : ""}
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {t.billingMode === "PAY_PER_REQUEST" ? "on-demand" : "provisioned"}
                    </td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">
                      {t.arn ? t.arn.split(":table/")[1] || t.arn : "-"}
                    </td>
                    <td className="px-5 py-3">
                      <ClusterStatusBadge status={t.status} error={t.lastError} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{timeAgo(t.createdAt)}</td>
                    <td className="sticky right-0 bg-card px-5 py-3 text-right">
                      <DestroyButton resource="tables" name={t.name} environment={t.environment} status={t.status} />
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
