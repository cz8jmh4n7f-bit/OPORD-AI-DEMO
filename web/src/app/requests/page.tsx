import Link from "next/link";
import { CheckCircle2, ClipboardCheck, Clock3, Layers, Plus, ShieldCheck, Workflow } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { EmptyState } from "@/components/empty-state";
import { RequestActions, RequestStatusBadge } from "@/components/request-actions";
import { StatCard } from "@/components/stat-card";
import { fetchRequests } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

export const metadata = { title: "Requests" };

export default async function RequestsPage() {
  const requests = await fetchRequests();
  const open = requests.filter((r) => ["pending_approval", "approved", "provisioning"].includes(r.status)).length;
  const pendingApproval = requests.filter((r) => r.status === "pending_approval").length;
  const provisioning = requests.filter((r) => r.status === "provisioning").length;
  const completed = requests.filter((r) => r.status === "completed").length;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Requests"
        description="Lifecycle view for service requests from submission through approval, provisioning, and ownership."
      >
        <Link href="/catalog" className={button({ size: "sm" })}>
          <Plus className="size-4" />
          Request service
        </Link>
      </PageHeader>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard icon={Workflow} label="Open requests" value={open} hint="Approval or provisioning active" />
        <StatCard icon={ShieldCheck} label="Pending approval" value={pendingApproval} hint="Needs platform decision" />
        <StatCard icon={Clock3} label="Provisioning" value={provisioning} hint="Provider work in progress" />
        <StatCard icon={CheckCircle2} label="Completed" value={completed} hint="Handed to owners" />
      </div>

      {requests.length === 0 ? (
        <Card>
          <EmptyState
            icon={Layers}
            title="No requests yet"
            description="Start from the catalog to request a governed infrastructure service."
            action={{ href: "/catalog", label: "Browse catalog" }}
          />
        </Card>
      ) : (
        <Card className="overflow-hidden p-0">
          <div className="border-b border-border px-5 py-4">
            <div className="flex items-center gap-2">
              <ClipboardCheck className="size-4 text-primary" />
              <h2 className="text-sm font-semibold tracking-tight">Request history</h2>
            </div>
            <p className="mt-1 text-sm text-muted-foreground">
              Existing backend request records. Approval and provisioning actions stay on the current API.
            </p>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
                  <th scope="col" className="px-5 py-3 font-medium">Name</th>
                  <th scope="col" className="px-5 py-3 font-medium">Requester</th>
                  <th scope="col" className="px-5 py-3 font-medium">Kind</th>
                  <th scope="col" className="px-5 py-3 font-medium">Provider</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Ticket</th>
                  <th scope="col" className="px-5 py-3 font-medium">Created</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {requests.map((r) => (
                  <tr key={r.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <Link href={`/requests/${r.name}`} className="font-medium text-foreground hover:text-primary">
                        {r.name}
                      </Link>
                      <div className="text-xs text-muted-foreground">{r.environment}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{r.requester || "-"}</td>
                    <td className="px-5 py-3">
                      <span className="rounded-md bg-muted px-2 py-0.5 font-mono text-xs text-muted-foreground">{r.kind}</span>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{r.provider}</td>
                    <td className="px-5 py-3">
                      <RequestStatusBadge status={r.status} />
                    </td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">{r.ticketRef || "-"}</td>
                    <td className="px-5 py-3 text-muted-foreground">{timeAgo(r.createdAt)}</td>
                    <td className="sticky right-0 bg-card px-5 py-3">
                      <RequestActions name={r.name} environment={r.environment} status={r.status} />
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
