import Link from "next/link";
import { CountUp } from "@/components/count-up";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import {
  fetchAIAudit,
  fetchAIBudgets,
  fetchAIInstances,
  fetchAIPolicies,
  fetchAIProviders,
  fetchAIQuotas,
  fetchAIRequests,
  fetchAIServices,
} from "@/lib/api";
import { formatDate } from "@/lib/utils";

export const metadata = { title: "Overview" };

type BadgeVariant = "default" | "primary" | "success" | "warning" | "danger" | "info";

// auditTone colors an audit action so blocks/failures read red and grants read green.
function auditTone(action: string): BadgeVariant {
  const a = action.toLowerCase();
  if (a.includes("blocked") || a.includes("failed") || a.includes("rejected") || a.includes("revoked")) return "danger";
  if (a.includes("warning")) return "warning";
  if (a.includes("created") || a.includes("granted") || a.includes("checked") || a.includes("synced")) return "success";
  return "info";
}

// A status dot for the posture bar: gray when nothing's set, green when active,
// red when something needs attention.
function Dot({ tone }: { tone: "idle" | "active" | "alert" }) {
  return (
    <span
      aria-hidden
      className={cn(
        "inline-block size-1.5 rounded-full",
        tone === "alert" ? "bg-danger" : tone === "active" ? "bg-success" : "bg-faint",
      )}
    />
  );
}

export default async function AIOverviewPage() {
  const [providers, services, instances, requests, budgets, quotas, policies, audit] = await Promise.all([
    fetchAIProviders(),
    fetchAIServices(),
    fetchAIInstances(),
    fetchAIRequests(),
    fetchAIBudgets(),
    fetchAIQuotas(),
    fetchAIPolicies(),
    fetchAIAudit(),
  ]);

  const activeAccess = instances.filter((i) => i.status === "active").length;
  const pending = requests.filter((r) => r.status === "pending_approval").length;
  const activePolicies = policies.filter((p) => p.status === "active").length;
  const blockingQuotas = quotas.filter((q) => q.enforcement === "block").length;
  const budgetsAtRisk = budgets.filter((b) => b.status === "warning" || b.status === "hard_limit").length;
  const spend = budgets.reduce((sum, b) => sum + (b.actualUsd || 0), 0);

  const cell = "rounded-lg border border-border bg-surface-2 p-4 card-hover";
  const cellLabel = "text-[11px] font-medium uppercase tracking-[0.06em] text-muted-foreground";

  return (
    <div className="space-y-5">
      <div>
        <h1 className="text-[18px] font-medium tracking-tight text-foreground">AI workspace</h1>
        <p className="mt-1 text-[13px] text-muted-foreground">
          Governed access to AI services - request, approve, meter, and audit, on the platform you already run.
        </p>
      </div>

      {/* Asymmetric hero: tracked spend dominates; providers + pending ride alongside. */}
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <div className={cn(cell, "flex flex-col sm:col-span-2")}>
          <span className={cellLabel}>Tracked spend</span>
          <div
            className={cn(
              "mt-3 text-[36px] font-semibold leading-none tabular-nums",
              spend === 0 ? "text-muted-foreground" : "text-foreground",
            )}
          >
            <CountUp value={`$${spend.toFixed(2)}`} />
          </div>
          <p className="mt-1.5 text-xs text-faint">Across all budget scopes</p>
          <div className="mt-auto flex flex-wrap gap-x-4 gap-y-1 pt-4 text-xs text-faint">
            <span>
              Active access <span className="text-muted-foreground tabular-nums">{activeAccess}</span>
            </span>
            <span>
              Catalog <span className="text-muted-foreground tabular-nums">{services.length}</span>
            </span>
          </div>
        </div>

        <div className={cn(cell, "flex flex-col")}>
          <span className={cellLabel}>AI providers</span>
          <div
            className={cn(
              "mt-3 text-[36px] font-semibold leading-none tabular-nums",
              providers.length === 0 ? "text-muted-foreground" : "text-foreground",
            )}
          >
            <CountUp value={providers.length} />
          </div>
          <p className="mt-1.5 text-xs text-faint">Backends</p>
        </div>

        <div className={cn(cell, "flex flex-col")}>
          <span className={cellLabel}>Pending requests</span>
          <div
            className={cn(
              "mt-3 text-[36px] font-semibold leading-none tabular-nums",
              pending === 0 ? "text-muted-foreground" : "text-foreground",
            )}
          >
            <CountUp value={pending} />
          </div>
          <Link href="/ai/requests" className="mt-auto pt-3 text-xs text-faint transition-colors hover:text-foreground">
            Awaiting approval →
          </Link>
        </div>
      </div>

      {/* Governance posture: one horizontal line, not four cards. */}
      <div className="flex flex-wrap items-center gap-x-5 gap-y-2 rounded-lg border border-border bg-surface-2 px-4 py-3 text-[13px]">
        <span className="text-[11px] font-medium uppercase tracking-[0.06em] text-faint">Governance posture</span>
        <span className="flex items-center gap-2 text-muted-foreground">
          <Dot tone={activePolicies > 0 ? "active" : "idle"} />
          Policies <span className="text-foreground tabular-nums">{activePolicies}</span>
        </span>
        <span className="flex items-center gap-2 text-muted-foreground">
          <Dot tone={blockingQuotas > 0 ? "active" : "idle"} />
          Quotas <span className="text-foreground tabular-nums">{blockingQuotas}</span>
        </span>
        <span className="flex items-center gap-2 text-muted-foreground">
          <Dot tone={budgetsAtRisk > 0 ? "alert" : "idle"} />
          Budgets at risk <span className="text-foreground tabular-nums">{budgetsAtRisk}</span>
        </span>
        <span className="flex items-center gap-2 text-muted-foreground">
          <Dot tone={spend > 0 ? "active" : "idle"} />
          <span className="text-foreground tabular-nums">{`$${spend.toFixed(2)}`}</span> tracked
        </span>
      </div>

      {/* Recent activity is the dominant element: full width. */}
      <section className="rounded-lg border border-border bg-surface-2">
        <div className="flex items-center justify-between px-4 py-3">
          <h2 className="text-[11px] font-medium uppercase tracking-[0.06em] text-faint">Recent activity</h2>
          <Link href="/ai/audit" className="text-xs font-medium text-muted-foreground transition-colors hover:text-foreground">
            Full audit →
          </Link>
        </div>
        {audit.length === 0 ? (
          <div className="px-4 pb-6 pt-1">
            <p className="text-sm font-medium text-muted-foreground">No activity yet</p>
            <p className="mt-1 text-sm text-faint">Requests, approvals, and grants will appear here.</p>
          </div>
        ) : (
          <ul className="divide-y divide-border border-t border-border">
            {audit.slice(0, 8).map((e) => (
              <li key={e.id} className="flex items-start gap-3 px-4 py-2.5">
                <Badge variant={auditTone(e.action)}>{e.action.replace(/_/g, " ")}</Badge>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[13px] text-foreground">{e.message}</p>
                  <p className="mt-0.5 text-xs text-faint">
                    {e.actor} · {formatDate(e.createdAt)}
                  </p>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
