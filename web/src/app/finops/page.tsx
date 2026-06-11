import Link from "next/link";
import { Activity, ArrowDownRight, ArrowUpRight, CalendarClock, CircleDollarSign, Cloud, Gauge, Landmark, Receipt, Scale, ShieldAlert, Tags, TrendingUp, WalletCards } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { StatCard } from "@/components/stat-card";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { CostTrend } from "@/components/cost-trend";
import { CostDonut } from "@/components/cost-donut";
import { CloudTiles } from "@/components/cloud-tiles";
import { FinOpsControls } from "@/components/finops-controls";
import { FinopsTabs } from "@/components/finops-tabs";
import { fetchFinOps } from "@/lib/api";

function usd(n: number): string {
  return "$" + n.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function Breakdown({
  title,
  items,
  empty,
}: {
  title: string;
  items: { name: string; monthlyUsd: number }[];
  empty: string;
}) {
  const max = Math.max(...items.map((i) => i.monthlyUsd), 0);
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {items.length === 0 && <p className="text-sm text-muted-foreground">{empty}</p>}
        {items.map((item) => {
          const width = max > 0 ? Math.max(6, Math.round((item.monthlyUsd / max) * 100)) : 0;
          return (
            <div key={item.name} className="space-y-1.5">
              <div className="flex items-center justify-between gap-3 text-sm">
                <span className="truncate font-medium">{item.name}</span>
                <span className="shrink-0 tabular-nums text-muted-foreground">{usd(item.monthlyUsd)}</span>
              </div>
              <div className="h-2 overflow-hidden rounded-full bg-muted">
                <div className="h-full rounded-full bg-primary" style={{ width: `${width}%` }} />
              </div>
            </div>
          );
        })}
      </CardContent>
    </Card>
  );
}

function statusClass(status: string): string {
  if (status === "over" || status === "blocker") return "bg-danger/10 text-danger";
  if (status === "risk" || status === "warn") return "bg-warning/10 text-warning";
  return "bg-success/10 text-success";
}

export const metadata = { title: "FinOps" };

export default async function FinOpsPage({
  searchParams,
}: {
  searchParams: Promise<{ provider?: string; account?: string; days?: string }>;
}) {
  const sp = await searchParams;
  const provider = sp.provider ?? "";
  const account = sp.account ?? "";
  const parsedDays = Number.parseInt(sp.days ?? "", 10);
  const days = Number.isFinite(parsedDays) && parsedDays > 0 ? parsedDays : 30;

  const report = await fetchFinOps(provider, account, days);
  const possibleSavings = report.savingsOpportunities.reduce((sum, item) => sum + item.savingsUsd, 0);
  const allocation = report.allocationCoverage;
  const actuals = report.actuals;
  const windowDays = report.windowDays ?? days;

  // Estimate (OPORD inventory) vs forecast (Cost Explorer run-rate) delta.
  // totalUsd is OPORD's own estimate; projectedMonthlyUsd gets overwritten with the
  // real run-rate forecast when actuals exist, so compare against totalUsd. When a
  // single cloud is selected, scope the estimate to that provider's resources - else
  // a GLOBAL estimate would be compared to one cloud's forecast (the -$23 bug).
  const estimateUsd = provider
    ? report.providerSpend.find((p) => p.name === provider)?.monthlyUsd ?? 0
    : report.totalUsd;
  const delta = actuals ? actuals.forecastUsd - estimateUsd : 0;
  const deltaOverEstimate = delta > 0; // forecast above estimate = spending more than projected

  // --- Hero: real-spend KPIs when actuals exist, else the estimate KPIs. ---
  const hero = actuals ? (
    <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
      <StatCard icon={CircleDollarSign} label="Month-to-date" value={usd(actuals.mtdUsd)} hint="billed so far this month" />
      <StatCard icon={CalendarClock} label="Forecast (run-rate)" value={usd(actuals.forecastUsd)} hint="end of month" accent="bg-info/10 text-info" />
      <StatCard icon={Activity} label="Daily run-rate" value={usd(actuals.dailyRunRate)} hint="recent avg/day" accent="bg-warning/10 text-warning" />
      <StatCard
        icon={Scale}
        label="Estimate vs actual"
        value={`${deltaOverEstimate ? "+" : "-"}${usd(Math.abs(delta))}`}
        hint={`estimate ${usd(estimateUsd)} vs forecast ${usd(actuals.forecastUsd)}`}
        accent={deltaOverEstimate ? "bg-danger/10 text-danger" : "bg-success/10 text-success"}
      />
    </div>
  ) : (
    <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
      <StatCard icon={CircleDollarSign} label="Projected monthly" value={usd(report.projectedMonthlyUsd ?? report.totalUsd)} hint={`${usd(report.dailyRunRateUsd ?? 0)} / day`} />
      <StatCard icon={Cloud} label="Managed resources" value={report.activeResources} hint="active inventory" accent="bg-info/10 text-info" />
      <StatCard icon={ArrowDownRight} label="Possible savings" value={usd(possibleSavings)} hint={`${report.savingsOpportunities.length} opportunities`} accent="bg-success/10 text-success" />
      <StatCard icon={Tags} label="Allocation coverage" value={`${allocation.coveragePct ?? 0}%`} hint="owner/project/cost center" accent="bg-warning/10 text-warning" />
    </div>
  );

  // --- Overview tab: the actuals visuals (when present) + estimate spend split. ---
  const overview = (
    <>
      {actuals && (
        <>
          <div className="grid grid-cols-2 gap-4 lg:grid-cols-3">
            <StatCard icon={Cloud} label="Managed resources" value={report.activeResources} hint="active inventory" accent="bg-info/10 text-info" />
            <StatCard icon={ArrowDownRight} label="Possible savings" value={usd(possibleSavings)} hint={`${report.savingsOpportunities.length} opportunities`} accent="bg-success/10 text-success" />
            <StatCard icon={Tags} label="Allocation coverage" value={`${allocation.coveragePct ?? 0}%`} hint="owner/project/cost center" accent="bg-warning/10 text-warning" />
          </div>
          <Card>
            <CardHeader>
              <CardTitle>
                Daily spend ({account || provider || "all clouds"}, last {windowDays} days)
              </CardTitle>
            </CardHeader>
            <CardContent>
              <CostTrend points={actuals.daily} />
            </CardContent>
          </Card>
          {!provider && actuals.byCloud && actuals.byCloud.length > 1 && (
            <Card>
              <CardHeader>
                <CardTitle>Spend by cloud</CardTitle>
                <CardDescription>Real billed spend across connected providers.</CardDescription>
              </CardHeader>
              <CardContent>
                <CostDonut
                  items={actuals.byCloud.map((b) => ({ name: b.name ? b.name : b.key, usd: b.usd }))}
                  empty="No billed cloud spend."
                />
              </CardContent>
            </Card>
          )}
          <div className="grid gap-6 lg:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle>Spend by account</CardTitle>
              </CardHeader>
              <CardContent>
                <CostDonut
                  items={actuals.byAccount.map((b) => ({ name: b.name ? b.name : b.key, usd: b.usd }))}
                  empty="No billed account spend in this window."
                />
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle>Spend by service</CardTitle>
              </CardHeader>
              <CardContent>
                <CostDonut
                  items={actuals.byService.map((b) => ({ name: b.key, usd: b.usd }))}
                  empty="No billed service spend in this window."
                />
              </CardContent>
            </Card>
          </div>
        </>
      )}
      <div className="grid gap-6 lg:grid-cols-2">
        <Breakdown title="Estimated spend by provider" items={report.providerSpend} empty="No active estimated provider spend yet." />
        <Breakdown title="Estimated spend by environment" items={report.environmentSpend} empty="No active estimated environment spend yet." />
      </div>
    </>
  );

  // --- Breakdown tab: deeper estimate cuts + allocation detail. ---
  const breakdown = (
    <>
      <div className="grid gap-6 lg:grid-cols-2">
        <Breakdown title="Estimated spend by resource kind" items={report.kindSpend ?? []} empty="No active estimated kind spend yet." />
        <Card>
          <CardHeader>
            <CardTitle>Unit economics</CardTitle>
            <CardDescription>Environment cost per managed resource.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {report.unitMetrics.length === 0 && <p className="text-sm text-muted-foreground">No unit metrics yet.</p>}
            {report.unitMetrics.map((item) => (
              <div key={item.name} className="flex items-center justify-between gap-3 rounded-lg border border-border bg-muted/30 p-3">
                <div className="flex items-center gap-2">
                  <Gauge className="size-4 text-primary" />
                  <div>
                    <div className="text-sm font-medium">{item.name}</div>
                    <div className="text-xs text-muted-foreground">{item.resources} resources</div>
                  </div>
                </div>
                <div className="text-right text-sm tabular-nums">
                  <div>{usd(item.monthlyUsd)}</div>
                  <div className="text-xs text-muted-foreground">{usd(item.avgUsd)} avg</div>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Allocation coverage</CardTitle>
          <CardDescription>How much inventory can be attributed to owners and teams.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 text-sm sm:grid-cols-2 lg:grid-cols-4">
          {[
            ["Owner", allocation.ownerTagged],
            ["Project", allocation.projectTagged],
            ["Cost center", allocation.costCenterTagged],
            ["TTL protected", allocation.ttlProtected],
          ].map(([label, value]) => (
            <div key={label} className="flex items-center justify-between rounded-lg border border-border bg-muted/30 p-3">
              <span className="text-muted-foreground">{label}</span>
              <span className="font-medium tabular-nums">{value} / {allocation.resources}</span>
            </div>
          ))}
        </CardContent>
      </Card>
    </>
  );

  // --- Optimize tab: anomalies + budgets + guardrails + savings. ---
  const optimize = (
    <>
      {actuals && actuals.anomalies.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Spend anomalies</CardTitle>
            <CardDescription>Days where spend ran well above the trailing baseline.</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-3 sm:grid-cols-2">
            {actuals.anomalies.map((a) => (
              <div key={a.date} className="flex items-center justify-between gap-3 rounded-lg border border-border bg-muted/30 p-3">
                <div className="flex items-center gap-2">
                  <Activity className="size-4 shrink-0 text-warning" />
                  <span className="text-sm font-medium tabular-nums">{a.date}</span>
                  <span className="rounded-md bg-warning/10 px-1.5 py-0.5 text-[10px] font-medium uppercase text-warning">
                    {a.factor.toFixed(1)}×
                  </span>
                </div>
                <div className="text-right text-sm tabular-nums">
                  <div className="font-medium">{usd(a.usd)}</div>
                  <div className="text-xs text-muted-foreground">vs {usd(a.baselineUsd)} baseline</div>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Budget guardrails</CardTitle>
          <CardDescription>Default environment budgets until team budgets are configured.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 md:grid-cols-2">
          {report.budgets.length === 0 && <p className="text-sm text-muted-foreground">No active budget usage yet.</p>}
          {report.budgets.map((budget) => (
            <div key={`${budget.scope}-${budget.name}`} className="grid gap-3 rounded-lg border border-border bg-muted/30 p-3 md:grid-cols-[1fr_auto]">
              <div>
                <div className="flex items-center gap-2">
                  <WalletCards className="size-4 text-primary" />
                  <span className="text-sm font-medium">{budget.name}</span>
                  <span className={`rounded-md px-1.5 py-0.5 text-[10px] font-medium uppercase ${statusClass(budget.status)}`}>{budget.status}</span>
                </div>
                <div className="mt-2 h-2 overflow-hidden rounded-full bg-background">
                  <div className="h-full rounded-full bg-primary" style={{ width: `${Math.min(100, budget.usagePct)}%` }} />
                </div>
              </div>
              <div className="text-right text-sm tabular-nums">
                <div className="font-medium">{usd(budget.actualUsd)} / {usd(budget.limitUsd)}</div>
                <div className="text-xs text-muted-foreground">{budget.usagePct.toFixed(1)}% used</div>
              </div>
            </div>
          ))}
        </CardContent>
      </Card>

      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Guardrails</CardTitle>
            <CardDescription>Issues OPORD should warn or block on before more provisioning.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {report.guardrails.length === 0 && <p className="text-sm text-muted-foreground">No guardrail findings yet.</p>}
            {report.guardrails.map((item) => (
              <div key={`${item.resource}-${item.kind}-${item.message}`} className="rounded-lg border border-border bg-muted/30 p-3">
                <div className="flex items-start gap-2">
                  <ShieldAlert className="mt-0.5 size-4 shrink-0 text-primary" />
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="truncate text-sm font-medium">{item.resource}</span>
                      <span className={`rounded-md px-1.5 py-0.5 text-[10px] font-medium uppercase ${statusClass(item.severity)}`}>{item.severity}</span>
                    </div>
                    <p className="mt-1 text-xs text-muted-foreground">{item.message}</p>
                    <p className="mt-1 text-xs text-foreground">{item.action}</p>
                  </div>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Savings backlog</CardTitle>
            <CardDescription>Estimated actions to review, ordered by possible monthly impact.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {report.savingsOpportunities.length === 0 && <p className="text-sm text-muted-foreground">No savings opportunities yet.</p>}
            {report.savingsOpportunities.map((item) => (
              <div key={`${item.resource}-${item.action}`} className="rounded-lg border border-border bg-muted/30 p-3">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="text-sm font-medium">{item.resource}</div>
                    <div className="mt-1 text-xs text-muted-foreground">{item.kind} · {item.provider} · {item.confidence} confidence</div>
                  </div>
                  <div className="shrink-0 text-right">
                    <div className="text-sm font-semibold tabular-nums">{usd(item.savingsUsd)}</div>
                    <div className="text-[10px] uppercase text-muted-foreground">save/mo</div>
                  </div>
                </div>
                <p className="mt-2 text-xs text-muted-foreground">{item.action}</p>
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
    </>
  );

  // --- Setup & guidance tab: FOCUS adoption, phases, recommendations. ---
  const setup = (
    <>
      <Card>
        <CardHeader>
          <CardTitle>FOCUS cloud setup</CardTitle>
          <CardDescription>Use provider-native exports first, then normalize billing into a shared cost and usage language.</CardDescription>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-3 lg:grid-cols-3">
          {report.focusGuides.map((guide) => (
            <a
              key={guide.cloud}
              href={guide.url}
              target="_blank"
              rel="noreferrer"
              className="group rounded-lg border border-border p-4 transition-colors hover:border-primary/40 hover:bg-muted"
            >
              <div className="flex items-start justify-between gap-3">
                <div>
                  <div className="text-sm font-semibold">{guide.cloud}</div>
                  <div className="mt-1 text-xs text-muted-foreground">FOCUS {guide.focusVersion} · {guide.status}</div>
                </div>
                <ArrowUpRight className="size-4 shrink-0 text-muted-foreground group-hover:text-primary" />
              </div>
              <div className="mt-4 space-y-3 text-xs text-muted-foreground">
                <p>
                  <span className="font-medium text-foreground">Export:</span> {guide.export}
                </p>
                <p>
                  <span className="font-medium text-foreground">Analyze:</span> {guide.analytics}
                </p>
                <p>
                  <span className="font-medium text-foreground">OPORD:</span> {guide.opordReadiness}
                </p>
              </div>
            </a>
          ))}
        </CardContent>
      </Card>

      <div className="grid gap-6 lg:grid-cols-3">
        {report.phases.map((phase) => (
          <Card key={phase.name}>
            <CardHeader>
              <CardTitle>{phase.name}</CardTitle>
              <CardDescription>{phase.description}</CardDescription>
            </CardHeader>
            <CardContent>
              <ul className="space-y-2 text-sm text-muted-foreground">
                {phase.actions.map((action) => (
                  <li key={action} className="flex gap-2">
                    <span className="mt-2 size-1.5 shrink-0 rounded-full bg-primary" />
                    <span>{action}</span>
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>
        ))}
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Next controls to wire into OPORD</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3 md:grid-cols-2">
          {report.recommendations.map((rec) => (
            <div key={rec} className="flex gap-3 rounded-lg border border-border bg-muted/30 p-3 text-sm text-muted-foreground">
              <Landmark className="mt-0.5 size-4 shrink-0 text-primary" />
              <span>{rec}</span>
            </div>
          ))}
        </CardContent>
      </Card>

      <div className="flex flex-wrap gap-3">
        <Link href="/cost" className="text-sm font-medium text-primary hover:underline">
          Open raw cost estimate
        </Link>
        <a href="https://focus.finops.org/" target="_blank" rel="noreferrer" className="text-sm font-medium text-primary hover:underline">
          FOCUS specification
        </a>
        <a href="https://www.finops.org/framework/" target="_blank" rel="noreferrer" className="text-sm font-medium text-primary hover:underline">
          FinOps Framework
        </a>
      </div>
    </>
  );

  return (
    <div className="space-y-6">
      <PageHeader
        title="FinOps"
        description="Cost allocation, budget guardrails, savings backlog, and FOCUS adoption for OPORD-managed infrastructure."
      >
        <FinOpsControls provider={provider} accounts={actuals?.accounts ?? []} account={account} days={days} />
      </PageHeader>

      {report.clouds && report.clouds.length > 0 && (
        <CloudTiles clouds={report.clouds} active={provider} days={days} />
      )}

      {report.actualsError && !actuals && (
        <Card className="border-warning/40 bg-warning/5">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Receipt className="size-5 text-warning" />
              Connect AWS Cost Explorer for live spend
            </CardTitle>
            <CardDescription>
              OPORD is showing <span className="font-medium text-foreground">estimates</span> below. Grant{" "}
              <code className="rounded bg-muted px-1 py-0.5 text-xs">ce:GetCostAndUsage</code> to OPORD&rsquo;s AWS
              credentials to unlock real billed cost - actuals by account and service, daily trend, run-rate forecast,
              and spend anomalies.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <p className="text-xs text-muted-foreground">{report.actualsError}</p>
          </CardContent>
        </Card>
      )}

      {actuals && (
        <div className="flex items-center gap-2">
          <TrendingUp className="size-5 text-primary" />
          <h2 className="text-lg font-semibold tracking-tight">Actual cloud spend</h2>
          <span className="text-xs text-muted-foreground">
            {report.actualsSource ?? "cloud billing"} · {actuals.currency}
          </span>
        </div>
      )}

      {hero}

      <FinopsTabs
        tabs={[
          { id: "overview", label: "Overview", content: overview },
          { id: "breakdown", label: "Breakdown", content: breakdown },
          { id: "optimize", label: "Optimize", content: optimize },
          { id: "setup", label: "Setup & guidance", content: setup },
        ]}
      />
    </div>
  );
}
