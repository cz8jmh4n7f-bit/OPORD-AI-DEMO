import Link from "next/link";
import { Layers, Plus } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { EmptyState } from "@/components/empty-state";
import { ClusterStatusBadge } from "@/components/status-badge";
import { DestroyButton } from "@/components/destroy-button";
import { fetchEnvironments } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

export const metadata = { title: "Environments" };

export default async function EnvironmentsPage() {
  const envs = await fetchEnvironments();

  return (
    <div className="space-y-6">
      <PageHeader title="Environments" description="Complete environments composed from blueprints.">
        <Link href="/environments/new" className={button({ size: "sm" })}>
          <Plus className="size-4" />
          New environment
        </Link>
      </PageHeader>

      {envs.length === 0 ? (
        <Card>
          <EmptyState
            icon={Layers}
            title="No environments yet"
            description="Compose a full environment from a blueprint - clusters, VMs and databases provisioned as one unit."
            action={{ href: "/environments/new", label: "New environment" }}
          />
        </Card>
      ) : (
        <Card className="overflow-hidden p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
                  <th scope="col" className="px-5 py-3 font-medium">Name</th>
                  <th scope="col" className="px-5 py-3 font-medium">Blueprint</th>
                  <th scope="col" className="px-5 py-3 font-medium">Provider</th>
                  <th scope="col" className="px-5 py-3 font-medium">Components</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Created</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {envs.map((e) => (
                  <tr key={e.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <Link href={`/environments/${e.name}`} className="font-medium text-foreground hover:text-primary">
                        {e.name}
                      </Link>
                      <div className="text-xs text-muted-foreground">{e.environment}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{e.blueprint}</td>
                    <td className="px-5 py-3 text-muted-foreground">{e.provider}</td>
                    <td className="px-5 py-3 text-muted-foreground">{e.components.length}</td>
                    <td className="px-5 py-3">
                      <ClusterStatusBadge status={e.status} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{timeAgo(e.createdAt)}</td>
                    <td className="sticky right-0 bg-card px-5 py-3 text-right">
                      <DestroyButton resource="environments" name={e.name} environment={e.environment} status={e.status} />
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
