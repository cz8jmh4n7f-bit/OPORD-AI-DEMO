import { History } from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { fetchAIAudit } from "@/lib/api";
import { formatDate } from "@/lib/utils";

export const metadata = { title: "AI Audit" };

export default async function AIAuditPage() {
  const events = await fetchAIAudit();

  return (
    <div className="space-y-6">
      <PageHeader title="AI Audit" description="Durable audit events for AI provider, request, instance, and revoke actions." />

      {events.length === 0 ? (
        <Card>
          <EmptyState
            icon={History}
            title="No AI audit events"
            description="AI request, approval, provisioning, and revoke actions will appear here."
            action={{ href: "/ai/catalog", label: "AI catalog" }}
          />
        </Card>
      ) : (
        <Card className="overflow-hidden p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
                  <th scope="col" className="px-5 py-3 font-medium">Time</th>
                  <th scope="col" className="px-5 py-3 font-medium">Actor</th>
                  <th scope="col" className="px-5 py-3 font-medium">Subject</th>
                  <th scope="col" className="px-5 py-3 font-medium">Action</th>
                  <th scope="col" className="px-5 py-3 font-medium">Message</th>
                </tr>
              </thead>
              <tbody>
                {events.map((e) => (
                  <tr key={e.id} className="border-b border-border last:border-0 align-top hover:bg-muted/60">
                    <td className="px-5 py-3 text-muted-foreground">{formatDate(e.createdAt)}</td>
                    <td className="px-5 py-3 text-muted-foreground">{e.actor}</td>
                    <td className="px-5 py-3">
                      <div className="font-mono text-xs text-muted-foreground">{e.subjectType}</div>
                      <div className="max-w-48 truncate font-mono text-xs text-muted-foreground">{e.subjectId || "-"}</div>
                    </td>
                    <td className="px-5 py-3">
                      <span className="rounded-md bg-muted px-2 py-0.5 font-mono text-xs text-muted-foreground">{e.action}</span>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{e.message}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      {events.length >= 100 && (
        <p className="text-xs text-muted-foreground">Showing the most recent 100 events.</p>
      )}
    </div>
  );
}
