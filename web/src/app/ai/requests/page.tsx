import Link from "next/link";
import { CheckCircle2, Clock3, MessageSquarePlus, Sparkles } from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { PageHeader } from "@/components/page-header";
import { RequestActions, RequestStatusBadge } from "@/components/request-actions";
import { StatCard } from "@/components/stat-card";
import { button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { fetchAIRequests } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

export const metadata = { title: "AI Requests" };

export default async function AIRequestsPage() {
  const requests = await fetchAIRequests();
  const pending = requests.filter((r) => r.status === "pending_approval").length;
  const completed = requests.filter((r) => r.status === "completed").length;
  const active = requests.filter((r) => ["pending_approval", "approved", "provisioning"].includes(r.status)).length;

  return (
    <div className="space-y-6">
      <PageHeader title="AI Requests" description="AI access requests using OPORD's shared approval workflow.">
        <Link href="/ai/catalog" className={button({ size: "sm" })}>
          <Sparkles className="size-4" />
          Request AI access
        </Link>
      </PageHeader>

      <div className="grid gap-3 sm:grid-cols-3">
        <StatCard icon={MessageSquarePlus} label="Open" value={active} hint="Awaiting decision or provisioning" />
        <StatCard icon={Clock3} label="Pending approval" value={pending} hint="Operator action required" accent="bg-warning/10 text-warning" />
        <StatCard icon={CheckCircle2} label="Completed" value={completed} hint="Instance created" accent="bg-success/10 text-success" />
      </div>

      {requests.length === 0 ? (
        <Card>
          <EmptyState
            icon={Sparkles}
            title="No AI requests yet"
            description="Browse the AI service catalog to submit a governed mock access request."
            action={{ href: "/ai/catalog", label: "AI catalog" }}
          />
        </Card>
      ) : (
        <Card className="overflow-hidden p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
                  <th scope="col" className="px-5 py-3 font-medium">Request</th>
                  <th scope="col" className="px-5 py-3 font-medium">Requester</th>
                  <th scope="col" className="px-5 py-3 font-medium">Provider</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Instance</th>
                  <th scope="col" className="px-5 py-3 font-medium">Created</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {requests.map((r) => (
                  <tr key={r.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <div className="font-medium">{r.name}</div>
                      <div className="font-mono text-xs text-muted-foreground">{r.kind}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{r.requester || "-"}</td>
                    <td className="px-5 py-3 text-muted-foreground">{r.provider}</td>
                    <td className="px-5 py-3"><RequestStatusBadge status={r.status} /></td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">{r.resourceRef || "-"}</td>
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
