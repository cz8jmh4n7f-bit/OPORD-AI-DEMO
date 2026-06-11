import { BrainCircuit, CircleDollarSign, Hash } from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { PageHeader } from "@/components/page-header";
import { StatCard } from "@/components/stat-card";
import { Card } from "@/components/ui/card";
import { fetchAIUsage } from "@/lib/api";
import { formatDate } from "@/lib/utils";

export const metadata = { title: "AI Usage" };

function usd(n: number): string {
  return "$" + n.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

export default async function AIUsagePage() {
  const records = await fetchAIUsage();
  const totalCost = records.reduce((sum, r) => sum + r.costUsd, 0);
  const totalQuantity = records.reduce((sum, r) => sum + r.quantity, 0);

  return (
    <div className="space-y-6">
      <PageHeader title="AI Usage" description="Usage records metered or imported from AI providers." />

      <div className="grid gap-3 sm:grid-cols-3">
        <StatCard icon={BrainCircuit} label="Usage records" value={records.length} hint="Imported or mock records" />
        <StatCard icon={Hash} label="Quantity" value={totalQuantity.toLocaleString()} hint="Across all metrics" accent="bg-info/10 text-info" />
        <StatCard icon={CircleDollarSign} label="Cost" value={usd(totalCost)} hint="Mock USD usage cost" accent="bg-success/10 text-success" />
      </div>

      {records.length === 0 ? (
        <Card>
          <EmptyState
            icon={BrainCircuit}
            title="No AI usage yet"
            description="Approving a mock AI request creates a placeholder usage record."
            action={{ href: "/ai/catalog", label: "Request AI access" }}
          />
        </Card>
      ) : (
        <Card className="overflow-hidden p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
                  <th scope="col" className="px-5 py-3 font-medium">Provider</th>
                  <th scope="col" className="px-5 py-3 font-medium">Owner</th>
                  <th scope="col" className="px-5 py-3 font-medium">Workspace</th>
                  <th scope="col" className="px-5 py-3 font-medium">Metric</th>
                  <th scope="col" className="px-5 py-3 font-medium text-right">Quantity</th>
                  <th scope="col" className="px-5 py-3 font-medium text-right">Cost</th>
                  <th scope="col" className="px-5 py-3 font-medium">Period</th>
                </tr>
              </thead>
              <tbody>
                {records.map((r) => (
                  <tr key={r.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3 text-muted-foreground">{r.providerName}</td>
                    <td className="px-5 py-3 text-muted-foreground">{r.owner || "-"}</td>
                    <td className="px-5 py-3 text-muted-foreground">{r.workspace || "-"}</td>
                    <td className="px-5 py-3">
                      <span className="rounded-md bg-muted px-2 py-0.5 font-mono text-xs text-muted-foreground">{r.metric}</span>
                    </td>
                    <td className="px-5 py-3 text-right tabular-nums">
                      {r.quantity.toLocaleString()}
                      {r.unit && r.unit !== r.metric ? ` ${r.unit}` : ""}
                    </td>
                    <td className="px-5 py-3 text-right tabular-nums">
                      {(r.raw as Record<string, unknown> | undefined)?.source === "not_imported" ? (
                        <span className="text-muted-foreground">not imported</span>
                      ) : (
                        usd(r.costUsd)
                      )}
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{formatDate(r.periodStart)} - {formatDate(r.periodEnd)}</td>
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
