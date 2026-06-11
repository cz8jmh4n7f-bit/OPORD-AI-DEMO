import { CheckCircle2, ShieldAlert, ShieldCheck, TriangleAlert } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { StatCard } from "@/components/stat-card";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge, type BadgeVariant } from "@/components/ui/badge";
import { fetchCompliance } from "@/lib/api";
import type { ComplianceFailure, ComplianceSeverity } from "@/lib/types";

export const metadata = { title: "Compliance" };

function severityVariant(s: ComplianceSeverity): BadgeVariant {
  return s === "critical" ? "danger" : s === "warning" ? "warning" : "info";
}

// scoreAccent / barColor return LITERAL Tailwind classes (not interpolated) so
// the JIT compiler picks them up: green >= 90, amber >= 70, else red.
function scoreAccent(score: number): string {
  return score >= 90 ? "bg-success/10 text-success" : score >= 70 ? "bg-warning/10 text-warning" : "bg-danger/10 text-danger";
}

function barColor(score: number): string {
  return score >= 90 ? "bg-success" : score >= 70 ? "bg-warning" : "bg-danger";
}

function ScoreBars({
  title,
  description,
  rows,
  empty,
}: {
  title: string;
  description?: string;
  rows: { name: string; passed: number; failed: number; score: number }[];
  empty: string;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      <CardContent className="space-y-3">
        {rows.length === 0 && <p className="text-sm text-muted-foreground">{empty}</p>}
        {rows.map((r) => (
          <div key={r.name} className="space-y-1.5">
            <div className="flex items-center justify-between gap-3 text-sm">
              <span className="truncate font-medium capitalize">{r.name}</span>
              <span className="shrink-0 tabular-nums text-muted-foreground">
                {Math.round(r.score)}% · {r.passed}/{r.passed + r.failed}
              </span>
            </div>
            <div className="h-2 overflow-hidden rounded-full bg-muted">
              <div className={`h-full rounded-full ${barColor(r.score)}`} style={{ width: `${Math.max(2, Math.round(r.score))}%` }} />
            </div>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}

function FailureRow({ f }: { f: ComplianceFailure }) {
  return (
    <tr className="border-t border-border/60 align-top">
      <td className="py-3 pr-3">
        <Badge variant={severityVariant(f.severity)}>{f.severity}</Badge>
      </td>
      <td className="py-3 pr-3">
        <div className="font-medium">{f.subject}</div>
        <div className="text-xs text-muted-foreground">
          {f.kind} · {f.provider}
          {f.account ? ` · ${f.account}` : ""}
        </div>
      </td>
      <td className="py-3 pr-3">
        <div className="font-medium">{f.title}</div>
        <div className="text-xs text-muted-foreground">{f.message}</div>
      </td>
      <td className="py-3 text-sm text-muted-foreground">{f.remediation}</td>
    </tr>
  );
}

export default async function CompliancePage() {
  const sc = await fetchCompliance();
  const passing = sc.failed === 0 && sc.evaluated > 0;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Compliance"
        description="Guardrail posture across your inventory - tagging, cost, security, and reliability checks evaluated over every managed resource and landing-zone account."
      />

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          icon={passing ? ShieldCheck : ShieldAlert}
          label="Compliance score"
          value={`${Math.round(sc.score)}%`}
          hint={`${sc.passed}/${sc.evaluated} checks passing`}
          accent={scoreAccent(sc.score)}
        />
        <StatCard icon={ShieldAlert} label="Critical failing" value={sc.criticalFailing} hint="must-fix violations" accent="bg-danger/10 text-danger" />
        <StatCard icon={TriangleAlert} label="Warnings" value={sc.warningFailing} hint="should-fix findings" accent="bg-warning/10 text-warning" />
        <StatCard icon={CheckCircle2} label="Passing" value={sc.passed} hint={`of ${sc.evaluated} evaluated`} accent="bg-success/10 text-success" />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <ScoreBars
          title="By category"
          description="Worst-scoring concerns first."
          rows={sc.byCategory.map((c) => ({ name: c.category, passed: c.passed, failed: c.failed, score: c.score }))}
          empty="No resources evaluated."
        />
        <ScoreBars
          title="By account"
          description="Per landing zone - the provider default and each managed account/project/subscription."
          rows={sc.byAccount}
          empty="No resources evaluated."
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Findings</CardTitle>
          <CardDescription>
            {passing
              ? "Every check passes."
              : `${sc.failed} finding${sc.failed === 1 ? "" : "s"} - most severe first.`}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {sc.failures.length === 0 ? (
            <div className="flex items-center gap-2 rounded-lg border border-success/30 bg-success/5 px-4 py-6 text-sm text-muted-foreground">
              <ShieldCheck className="size-5 text-success" />
              {sc.evaluated === 0 ? "Nothing to evaluate yet - provision a resource to see its posture." : "All guardrails pass across the inventory."}
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left text-xs uppercase tracking-wide text-muted-foreground">
                    <th scope="col" className="pb-2 pr-3 font-medium">Severity</th>
                    <th scope="col" className="pb-2 pr-3 font-medium">Resource</th>
                    <th scope="col" className="pb-2 pr-3 font-medium">Check</th>
                    <th scope="col" className="pb-2 font-medium">Remediation</th>
                  </tr>
                </thead>
                <tbody>
                  {sc.failures.map((f, i) => (
                    <FailureRow key={`${f.checkId}-${f.subject}-${i}`} f={f} />
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
