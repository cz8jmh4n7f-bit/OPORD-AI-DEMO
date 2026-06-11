import Link from "next/link";
import { ClipboardCheck, ShieldCheck } from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { PageHeader } from "@/components/page-header";
import { RequestActions, RequestStatusBadge } from "@/components/request-actions";
import { Card } from "@/components/ui/card";
import { fetchRequests } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

export const metadata = { title: "Approvals" };

export default async function ApprovalsPage() {
  const requests = await fetchRequests();
  const approvals = requests.filter((request) => request.status === "pending_approval");

  return (
    <div className="space-y-6">
      <PageHeader
        title="Approvals"
        description="Requests waiting for governance decision before provisioning starts."
      />

      {approvals.length === 0 ? (
        <Card>
          <EmptyState
            icon={ShieldCheck}
            title="No approvals waiting"
            description="Pending approval requests will appear here with requester, provider, and environment context."
            action={{ href: "/requests", label: "View all requests" }}
          />
        </Card>
      ) : (
        <Card className="overflow-hidden p-0">
          <div className="border-b border-border px-5 py-4">
            <div className="flex items-center gap-2">
              <ClipboardCheck className="size-4 text-primary" />
              <h2 className="text-sm font-semibold tracking-tight">Approval queue</h2>
            </div>
            <p className="mt-1 text-sm text-muted-foreground">
              Approving starts provisioning through the existing request API.
            </p>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
                  <th scope="col" className="px-5 py-3 font-medium">Request</th>
                  <th scope="col" className="px-5 py-3 font-medium">Requester</th>
                  <th scope="col" className="px-5 py-3 font-medium">Service</th>
                  <th scope="col" className="px-5 py-3 font-medium">Provider</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Created</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Decision</th>
                </tr>
              </thead>
              <tbody>
                {approvals.map((request) => (
                  <tr key={request.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <Link href={`/requests/${request.name}`} className="font-medium text-foreground hover:text-primary">
                        {request.name}
                      </Link>
                      <div className="text-xs text-muted-foreground">{request.environment}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{request.requester || "-"}</td>
                    <td className="px-5 py-3">
                      <span className="rounded-md bg-muted px-2 py-0.5 font-mono text-xs text-muted-foreground">
                        {request.kind}
                      </span>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{request.provider}</td>
                    <td className="px-5 py-3">
                      <RequestStatusBadge status={request.status} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{timeAgo(request.createdAt)}</td>
                    <td className="sticky right-0 bg-card px-5 py-3">
                      <RequestActions name={request.name} environment={request.environment} status={request.status} />
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
