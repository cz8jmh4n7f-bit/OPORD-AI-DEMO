import Link from "next/link";
import { Inbox, Plus } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { EmptyState } from "@/components/empty-state";
import { ClusterStatusBadge } from "@/components/status-badge";
import { DestroyButton } from "@/components/destroy-button";
import { DeployTarget } from "@/components/deploy-target";
import { fetchQueues } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

export const metadata = { title: "Queues" };

export default async function QueuesPage() {
  const queues = await fetchQueues();

  return (
    <div className="space-y-6">
      <PageHeader
        title="Queues"
        description="Managed message queues (AWS SQS / Azure Service Bus / GCP Pub/Sub) with optional FIFO ordering and a dead-letter queue."
      >
        <Link href="/queues/new" className={button({ size: "sm" })}>
          <Plus className="size-4" />
          New queue
        </Link>
      </PageHeader>

      {queues.length === 0 ? (
        <Card>
          <EmptyState
            icon={Inbox}
            title="No queues yet"
            description="Provision a message queue from the catalog or CLI (opord queue create)."
            action={{ href: "/queues/new", label: "New queue" }}
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
                  <th scope="col" className="px-5 py-3 font-medium">Queue</th>
                  <th scope="col" className="px-5 py-3 font-medium">Type</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Created</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {queues.map((queue) => (
                  <tr key={queue.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <span className="font-medium">{queue.name}</span>
                      <div className="text-xs text-muted-foreground">{queue.environment}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {queue.provider}
                      <DeployTarget account={queue.targetAccount} />
                    </td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">{queue.queueName}</td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {queue.fifo ? "FIFO" : "standard"}
                      {queue.dlqEnabled ? " / DLQ" : ""}
                    </td>
                    <td className="px-5 py-3">
                      <ClusterStatusBadge status={queue.status} error={queue.lastError} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{timeAgo(queue.createdAt)}</td>
                    <td className="sticky right-0 bg-card px-5 py-3 text-right">
                      <DestroyButton resource="queues" name={queue.name} environment={queue.environment} status={queue.status} />
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
