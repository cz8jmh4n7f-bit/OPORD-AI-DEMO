import { PageHeader } from "@/components/page-header";
import { Card, CardContent } from "@/components/ui/card";
import { fetchCost } from "@/lib/api";

function usd(n: number): string {
  return "$" + n.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

export const metadata = { title: "Cost" };

export default async function CostPage() {
  const report = await fetchCost();

  return (
    <div className="space-y-6">
      <PageHeader title="Cost" description="Estimated monthly spend across active resources (rough; stacks excluded)." />

      <Card>
        <CardContent className="flex items-baseline justify-between p-5">
          <span className="text-sm text-muted-foreground">Total estimated monthly cost</span>
          <span className="text-3xl font-semibold tabular-nums">{usd(report.totalUsd)}</span>
        </CardContent>
      </Card>

      <Card className="overflow-hidden p-0">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
                <th scope="col" className="px-5 py-3 font-medium">Name</th>
                <th scope="col" className="px-5 py-3 font-medium">Kind</th>
                <th scope="col" className="px-5 py-3 font-medium">Provider</th>
                <th scope="col" className="px-5 py-3 font-medium">Environment</th>
                <th scope="col" className="px-5 py-3 font-medium">Status</th>
                <th scope="col" className="px-5 py-3 font-medium text-right">Est. $/mo</th>
              </tr>
            </thead>
            <tbody>
              {report.lines.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-5 py-8 text-center text-muted-foreground">
                    No active resources to cost.
                  </td>
                </tr>
              )}
              {report.lines.map((l) => (
                <tr key={`${l.kind}-${l.name}-${l.environment}`} className="border-b border-border last:border-0 hover:bg-muted/60">
                  <td className="px-5 py-3 font-medium">{l.name}</td>
                  <td className="px-5 py-3">
                    <span className="rounded-md bg-muted px-2 py-0.5 font-mono text-xs text-muted-foreground">{l.kind}</span>
                  </td>
                  <td className="px-5 py-3 text-muted-foreground">{l.provider || "-"}</td>
                  <td className="px-5 py-3 text-muted-foreground">{l.environment}</td>
                  <td className="px-5 py-3 text-muted-foreground">{l.status}</td>
                  <td className="px-5 py-3 text-right tabular-nums">{usd(l.monthlyUsd)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>
    </div>
  );
}
