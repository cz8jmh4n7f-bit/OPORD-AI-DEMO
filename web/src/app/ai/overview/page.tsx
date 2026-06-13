import Link from "next/link";
import {
  Bot,
  ChartNoAxesCombined,
  CircleDollarSign,
  History,
  MessageSquarePlus,
  Server,
  ShieldCheck,
  SlidersHorizontal,
  Sparkles,
} from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { PageHeader } from "@/components/page-header";
import { StatCard } from "@/components/stat-card";
import { Badge } from "@/components/ui/badge";
import { button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
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

export const metadata = { title: "AI workspace" };

type BadgeVariant = "default" | "primary" | "success" | "warning" | "danger" | "info";

// auditTone colors an audit action so blocks/failures read red and grants read green.
function auditTone(action: string): BadgeVariant {
  const a = action.toLowerCase();
  if (a.includes("blocked") || a.includes("failed") || a.includes("rejected") || a.includes("revoked")) return "danger";
  if (a.includes("warning")) return "warning";
  if (a.includes("created") || a.includes("granted") || a.includes("checked") || a.includes("synced")) return "success";
  return "info";
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

  return (
    <div className="space-y-6">
      <PageHeader
        title="AI workspace"
        description="Governed access to AI services - request, approve, meter, and audit, on the platform you already run."
      >
        <Link href="/ai/catalog" className={button({ size: "sm" })}>
          <Sparkles className="size-4" />
          Browse AI services
        </Link>
      </PageHeader>

      {/* The Phase-1 payoff, made visible: governance is actually enforced. */}
      <Card className="flex items-start gap-3 border-primary/30 bg-primary/5 p-4">
        <span className="mt-0.5 grid size-9 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">
          <ShieldCheck className="size-5" />
        </span>
        <div className="text-sm">
          <p className="font-medium text-foreground">Governance is enforced</p>
          <p className="mt-0.5 text-muted-foreground">
            Every AI request is checked against active <span className="text-foreground">policies</span>, seat{" "}
            <span className="text-foreground">quotas</span>, and <span className="text-foreground">budgets</span> before
            access is granted - blocked requests are refused with a reason and audited.
          </p>
        </div>
      </Card>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard icon={Server} label="AI providers" value={providers.length} hint="Registered backends" />
        <StatCard icon={Sparkles} label="Catalog services" value={services.length} hint="Requestable entitlements" />
        <StatCard
          icon={Bot}
          label="Active access"
          value={activeAccess}
          hint="Live instances"
          accent="bg-success/10 text-success"
        />
        <StatCard
          icon={MessageSquarePlus}
          label="Pending requests"
          value={pending}
          hint="Awaiting approval"
          accent={pending > 0 ? "bg-warning/10 text-warning" : undefined}
        />
      </div>

      <div className="space-y-3">
        <h2 className="text-sm font-semibold tracking-tight text-foreground">Governance posture</h2>
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <StatCard icon={ShieldCheck} label="Active policies" value={activePolicies} hint={`${policies.length} total`} />
          <StatCard
            icon={SlidersHorizontal}
            label="Blocking quotas"
            value={blockingQuotas}
            hint={`${quotas.length} quotas`}
          />
          <StatCard
            icon={CircleDollarSign}
            label="Budgets at risk"
            value={budgetsAtRisk}
            hint={`${budgets.length} budgets`}
            accent={budgetsAtRisk > 0 ? "bg-warning/10 text-warning" : undefined}
          />
          <StatCard
            icon={ChartNoAxesCombined}
            label="Tracked spend"
            value={`$${spend.toFixed(2)}`}
            hint="Across budget scopes"
          />
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-3">
        <Card className="lg:col-span-2 p-0">
          <div className="flex items-center justify-between border-b border-border px-5 py-3">
            <h2 className="flex items-center gap-2 text-sm font-semibold text-foreground">
              <History className="size-4 text-muted-foreground" />
              Recent activity
            </h2>
            <Link href="/ai/audit" className="text-xs font-medium text-primary hover:underline">
              Full audit
            </Link>
          </div>
          {audit.length === 0 ? (
            <EmptyState icon={History} title="No AI activity yet" description="Requests, approvals, and grants show up here." />
          ) : (
            <ul className="divide-y divide-border">
              {audit.slice(0, 6).map((e) => (
                <li key={e.id} className="flex items-start gap-3 px-5 py-3">
                  <Badge variant={auditTone(e.action)}>{e.action.replace(/_/g, " ")}</Badge>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm text-foreground">{e.message}</p>
                    <p className="mt-0.5 text-xs text-muted-foreground">
                      {e.actor} · {formatDate(e.createdAt)}
                    </p>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </Card>

        <Card className="p-0">
          <div className="border-b border-border px-5 py-3">
            <h2 className="text-sm font-semibold text-foreground">Jump to</h2>
          </div>
          <ul className="divide-y divide-border text-sm">
            {[
              { href: "/ai/requests", label: "Requests", icon: MessageSquarePlus },
              { href: "/ai/instances", label: "Active access", icon: Bot },
              { href: "/ai/budgets", label: "Budgets", icon: CircleDollarSign },
              { href: "/ai/quotas", label: "Quotas", icon: SlidersHorizontal },
              { href: "/ai/policies", label: "Policies", icon: ShieldCheck },
              { href: "/ai/providers", label: "Providers", icon: Server },
            ].map((l) => (
              <li key={l.href}>
                <Link href={l.href} className="flex items-center gap-3 px-5 py-2.5 text-foreground hover:bg-muted/60">
                  <l.icon className="size-4 text-muted-foreground" />
                  {l.label}
                </Link>
              </li>
            ))}
          </ul>
        </Card>
      </div>
    </div>
  );
}
