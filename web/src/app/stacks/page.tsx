import Link from "next/link";
import { Blocks, Plus } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { EmptyState } from "@/components/empty-state";
import { ClusterStatusBadge } from "@/components/status-badge";
import { DestroyButton } from "@/components/destroy-button";
import { DeployTarget } from "@/components/deploy-target";
import { fetchStacks } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

export const metadata = { title: "Stacks" };

export default async function StacksPage() {
  const stacks = await fetchStacks();

  return (
    <div className="space-y-6">
      <PageHeader title="Stacks" description="Arbitrary OpenTofu modules - provision anything the provider supports.">
        <Link href="/stacks/new" className={button({ size: "sm" })}>
          <Plus className="size-4" />
          New stack
        </Link>
      </PageHeader>

      {stacks.length === 0 ? (
        <Card>
          <EmptyState
            icon={Blocks}
            title="No stacks yet"
            description="Point OPORD at any OpenTofu root module to provision arbitrary resources the provider supports."
            action={{ href: "/stacks/new", label: "New stack" }}
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
                  <th scope="col" className="px-5 py-3 font-medium">Module</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Created</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {stacks.map((st) => (
                  <tr key={st.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <Link href={`/stacks/${st.name}`} className="font-medium text-foreground hover:text-primary">
                        {st.name}
                      </Link>
                      <div className="text-xs text-muted-foreground">{st.environment}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {st.provider}
                      <DeployTarget account={st.targetAccount} />
                    </td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">{st.moduleDir}</td>
                    <td className="px-5 py-3">
                      <ClusterStatusBadge status={st.status} error={st.lastError} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{timeAgo(st.createdAt)}</td>
                    <td className="sticky right-0 bg-card px-5 py-3 text-right">
                      <DestroyButton resource="stacks" name={st.name} environment={st.environment} status={st.status} />
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
