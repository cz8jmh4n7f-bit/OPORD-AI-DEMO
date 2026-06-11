import Link from "next/link";
import { KeyRound, Plus } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { EmptyState } from "@/components/empty-state";
import { ClusterStatusBadge } from "@/components/status-badge";
import { DestroyButton } from "@/components/destroy-button";
import { ProjectMembers } from "@/components/project-members";
import { fetchProjects } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

export const metadata = { title: "Projects" };

export default async function ProjectsPage() {
  const projects = await fetchProjects();

  return (
    <div className="space-y-6">
      <PageHeader
        title="Projects"
        description="Self-service cloud access: an AWS Identity Center group / Azure Entra group / GCP IAM role granted to a team, with members."
      >
        <Link href="/projects/new" className={button({ size: "sm" })}>
          <Plus className="size-4" />
          New project
        </Link>
      </PageHeader>

      {projects.length === 0 ? (
        <Card>
          <EmptyState
            icon={KeyRound}
            title="No projects yet"
            description="A project provisions an Identity Center group, permission set, and account assignment. Add members and OPORD reconciles their access - add or remove a user any time."
            action={{ href: "/projects/new", label: "New project" }}
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
                  <th scope="col" className="px-5 py-3 font-medium">Account</th>
                  <th scope="col" className="px-5 py-3 font-medium">Members</th>
                  <th scope="col" className="px-5 py-3 font-medium">Group</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Created</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {projects.map((p) => (
                  <tr key={p.id} className="border-b border-border last:border-0 align-top hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <span className="font-medium">{p.name}</span>
                      <div className="text-xs text-muted-foreground">{p.environment}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{p.provider}</td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">{p.accountId}</td>
                    <td className="px-5 py-3 min-w-[16rem]">
                      <ProjectMembers name={p.name} environment={p.environment} members={p.members} status={p.status} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{p.groupName || "-"}</td>
                    <td className="px-5 py-3">
                      <ClusterStatusBadge status={p.status} error={p.lastError} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{timeAgo(p.createdAt)}</td>
                    <td className="sticky right-0 bg-card px-5 py-3 text-right">
                      <DestroyButton resource="projects" name={p.name} environment={p.environment} status={p.status} />
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
