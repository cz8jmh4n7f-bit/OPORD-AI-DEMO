import { CalendarClock } from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { fetchAIRenewals } from "@/lib/api";
import { formatDate } from "@/lib/utils";

export const metadata = { title: "AI Renewals" };

function daysUntil(raw?: string): string {
  if (!raw) return "-";
  const diff = new Date(raw).getTime() - Date.now();
  return `${Math.ceil(diff / 86_400_000)}d`;
}

export default async function AIRenewalsPage() {
  const renewals = await fetchAIRenewals();

  return (
    <div className="space-y-6">
      <PageHeader title="AI Renewals" description="AI access expiring in the next 30 days for renewal or decommissioning." />

      {renewals.length === 0 ? (
        <Card>
          <EmptyState icon={CalendarClock} title="No upcoming renewals" description="Active AI access with an expiry date will appear here before it expires." />
        </Card>
      ) : (
        <Card className="overflow-hidden p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
                  <th scope="col" className="px-5 py-3 font-medium">Service</th>
                  <th scope="col" className="px-5 py-3 font-medium">Owner</th>
                  <th scope="col" className="px-5 py-3 font-medium">Workspace</th>
                  <th scope="col" className="px-5 py-3 font-medium">Expires</th>
                  <th scope="col" className="px-5 py-3 font-medium">Remaining</th>
                </tr>
              </thead>
              <tbody>
                {renewals.map((r) => (
                  <tr key={r.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <div className="font-medium">{r.serviceName}</div>
                      <div className="text-xs text-muted-foreground">{r.providerName}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{r.owner || "-"}</td>
                    <td className="px-5 py-3 text-muted-foreground">{r.workspace || "-"}</td>
                    <td className="px-5 py-3 text-muted-foreground">{r.expiresAt ? formatDate(r.expiresAt) : "-"}</td>
                    <td className="px-5 py-3"><Badge variant="warning">{daysUntil(r.expiresAt)}</Badge></td>
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
