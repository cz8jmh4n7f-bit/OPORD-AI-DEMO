import Link from "next/link";
import {
  AlertTriangle,
  CalendarClock,
  CheckCircle2,
  ClipboardList,
  DollarSign,
  Layers,
  ShieldAlert,
  Tags,
  UserCheck,
} from "lucide-react";
import { GetStarted } from "@/components/get-started";
import { PageHeader } from "@/components/page-header";
import { StatCard } from "@/components/stat-card";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { JobStatusBadge } from "@/components/status-badge";
import { RequestStatusBadge } from "@/components/request-actions";
import {
  fetchClusters,
  fetchVMs,
  fetchStacks,
  fetchEnvironments,
  fetchRequests,
  fetchCost,
  fetchJobs,
  fetchAccounts,
  fetchDatabases,
  fetchS3Buckets,
  fetchSecrets,
  fetchQueues,
  fetchCaches,
  fetchTables,
  fetchFunctions,
  fetchProviders,
  fetchAIProviders,
} from "@/lib/api";
import { timeAgo } from "@/lib/utils";

const liveStatuses = new Set(["ready", "degraded", "provisioning", "bootstrapping", "destroying"]);
const requestOpenStatuses = new Set(["pending_approval", "approved", "provisioning"]);

function isLive(status: string): boolean {
  return liveStatuses.has(status);
}

function usd(n: number): string {
  return "$" + n.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

export default async function DashboardPage() {
  const [clusters, vms, stacks, envs, requests, cost, jobs, accounts, databases, s3buckets, secrets, queues, caches, tables, functions, providers, aiProviders] =
    await Promise.all([
      fetchClusters(),
      fetchVMs(),
      fetchStacks(),
      fetchEnvironments(),
      fetchRequests(),
      fetchCost(),
      fetchJobs(),
      fetchAccounts(),
      fetchDatabases(),
      fetchS3Buckets(),
      fetchSecrets(),
      fetchQueues(),
      fetchCaches(),
      fetchTables(),
      fetchFunctions(),
      fetchProviders(),
      fetchAIProviders(),
    ]);

  const resourceInventory = [
    ...clusters,
    ...vms,
    ...stacks,
    ...envs,
    ...databases,
    ...s3buckets,
    ...secrets,
    ...queues,
    ...caches,
    ...tables,
    ...functions,
  ];
  const activeResources = resourceInventory.filter((r) => isLive(r.status)).length;
  const failedResources = resourceInventory.filter((r) => r.status === "failed" || r.status === "degraded").length;
  const openRequests = requests.filter((r) => requestOpenStatuses.has(r.status));
  const pendingApprovals = requests.filter((r) => r.status === "pending_approval");
  const failedJobs = jobs.filter((j) => j.status === "failed");
  const monthlyCost = usd(cost.totalUsd ?? 0);
  const missingOwner = cost.lines.filter((line) => !line.owner && isLive(line.status)).length;
  // eslint-disable-next-line react-hooks/purity -- server dashboard needs request-time TTL warning math.
  const nowMs = Date.now();
  const expiringSoon = vms.filter((vm) => {
    if (!vm.ttlHours || !isLive(vm.status)) return false;
    const expiresAt = new Date(vm.createdAt).getTime() + vm.ttlHours * 60 * 60 * 1000;
    const hoursLeft = (expiresAt - nowMs) / (60 * 60 * 1000);
    return hoursLeft > 0 && hoursLeft <= 72;
  });
  const ttlTracked = vms.some((vm) => !!vm.ttlHours);
  const activeEnvironments = envs.filter((env) => isLive(env.status)).length;
  const recentJobs = jobs.slice(0, 6);
  const projectSpend = cost.lines.reduce<Record<string, number>>((acc, line) => {
    const key = line.project || "Unallocated";
    acc[key] = (acc[key] ?? 0) + line.monthlyUsd;
    return acc;
  }, {});
  const topProjects = Object.entries(projectSpend)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 5);
  // First-run onboarding: show the checklist until something real exists. The
  // seeded MockAI provider doesn't count as a "connected" AI provider.
  const hasInfraProvider = providers.length > 0;
  const hasAIProvider = aiProviders.some((p) => p.type !== "mock_ai");
  const firstRun = resourceInventory.length === 0 && requests.length === 0;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Home"
        description="Request lifecycle, governance, cost, and operational health across OPORD-managed services."
      >
        <Link
          href="/catalog"
          className="inline-flex h-8 items-center justify-center rounded-lg bg-primary px-3 text-sm font-medium text-primary-foreground shadow-sm transition-colors hover:bg-primary/90"
        >
          Request service
        </Link>
      </PageHeader>

      {firstRun && (
        <GetStarted
          hasInfraProvider={hasInfraProvider}
          hasAIProvider={hasAIProvider}
          hasAnyRequest={requests.length > 0}
        />
      )}

      <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
        <StatCard icon={ClipboardList} label="Open requests" value={openRequests.length} hint={`${pendingApprovals.length} awaiting approval`} />
        <StatCard icon={UserCheck} label="Pending approvals" value={pendingApprovals.length} hint="governed before provisioning" accent="bg-warning/10 text-warning" />
        <StatCard icon={ShieldAlert} label="Needs attention" value={failedResources + failedJobs.length} hint={`${failedJobs.length} failed jobs`} accent="bg-danger/10 text-danger" />
        <StatCard icon={DollarSign} label="Estimated monthly" value={monthlyCost} hint={`${missingOwner} resources without owner`} accent="bg-info/10 text-info" />
      </div>

      <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
        <StatCard icon={Layers} label="Active environments" value={activeEnvironments} hint={`${activeResources} active resources`} accent="bg-success/10 text-success" />
        <StatCard icon={CalendarClock} label="Expiring soon" value={expiringSoon.length} hint={ttlTracked ? "next 72 hours" : "TTL data unavailable"} accent="bg-warning/10 text-warning" />
        <StatCard icon={Tags} label="Missing owner" value={missingOwner} hint="allocation metadata" accent="bg-warning/10 text-warning" />
        <StatCard icon={CheckCircle2} label="Managed accounts" value={accounts.filter((a) => isLive(a.status)).length} hint={`${accounts.length} total landing zones`} />
      </div>

      {(failedResources > 0 || failedJobs.length > 0 || missingOwner > 0) && (
        <div
          role="status"
          className="grid gap-3 rounded-xl border border-warning/30 bg-warning/10 px-4 py-3 text-sm text-warning md:grid-cols-[auto_1fr_auto]"
        >
          <AlertTriangle className="size-5 shrink-0" />
          <div>
            <div className="font-semibold">Governance attention required</div>
            <div className="text-warning/90">
              {failedResources + failedJobs.length} lifecycle issue{failedResources + failedJobs.length === 1 ? "" : "s"} and {missingOwner} unowned cost line{missingOwner === 1 ? "" : "s"} need review.
            </div>
          </div>
          <Link href="/requests" className="text-sm font-medium text-warning hover:underline">
            Review requests
          </Link>
        </div>
      )}

      <div className="grid gap-6 xl:grid-cols-3">
        <Card className="xl:col-span-2">
          <CardHeader className="flex-row items-center justify-between">
            <div>
              <CardTitle>Recent lifecycle activity</CardTitle>
              <CardDescription>Provisioning, reconciliation, and decommissioning jobs.</CardDescription>
            </div>
            <Link href="/jobs" className="text-xs font-medium text-primary hover:underline">
              View activity
            </Link>
          </CardHeader>
          <CardContent className="space-y-3.5">
            {recentJobs.length === 0 && <p className="text-sm text-muted-foreground">No lifecycle activity yet.</p>}
            {recentJobs.map((j) => (
              <div key={j.id} className="flex items-center justify-between gap-3 rounded-lg border border-border px-3 py-2">
                <div className="min-w-0">
                  <div className="truncate text-sm font-medium">{j.cluster}</div>
                  <div className="text-xs text-muted-foreground">
                    {j.operation} · {j.startedAt ? timeAgo(j.startedAt) : "queued"}
                  </div>
                </div>
                <JobStatusBadge status={j.status} />
              </div>
            ))}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex-row items-center justify-between">
            <div>
              <CardTitle>Approvals queue</CardTitle>
              <CardDescription>Requests waiting for a decision.</CardDescription>
            </div>
            <Link href="/approvals" className="text-xs font-medium text-primary hover:underline">
              Open approvals
            </Link>
          </CardHeader>
          <CardContent className="space-y-2">
            {pendingApprovals.length === 0 && <p className="text-sm text-muted-foreground">Nothing awaiting approval.</p>}
            {pendingApprovals.slice(0, 5).map((r) => (
              <Link
                key={r.id}
                href={`/requests/${r.name}`}
                className="flex items-center justify-between gap-3 rounded-lg border border-border px-3 py-2 transition-colors hover:bg-muted"
              >
                <div className="min-w-0">
                  <div className="truncate text-sm font-medium">{r.name}</div>
                  <div className="text-xs text-muted-foreground">
                    {r.kind} · {r.environment}
                  </div>
                </div>
                <RequestStatusBadge status={r.status} />
              </Link>
            ))}
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-6 xl:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Top projects by estimated usage</CardTitle>
            <CardDescription>Uses current estimate metadata; actual billing comes from Cost & usage.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {topProjects.length === 0 && <p className="text-sm text-muted-foreground">Project allocation metadata is unavailable.</p>}
            {topProjects.map(([project, monthly]) => (
              <div key={project} className="flex items-center justify-between gap-3 text-sm">
                <span className="truncate font-medium">{project}</span>
                <span className="shrink-0 tabular-nums text-muted-foreground">{usd(monthly)}</span>
              </div>
            ))}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Marketplace coverage</CardTitle>
            <CardDescription>Active service inventory by lifecycle area.</CardDescription>
          </CardHeader>
          <CardContent className="grid grid-cols-2 gap-3 text-sm">
            {[
              ["Compute", clusters.filter((c) => isLive(c.status)).length + vms.filter((v) => isLive(v.status)).length],
              ["Data", databases.filter((d) => isLive(d.status)).length + s3buckets.filter((b) => isLive(b.status)).length + caches.filter((c) => isLive(c.status)).length + tables.filter((t) => isLive(t.status)).length],
              ["App services", functions.filter((f) => isLive(f.status)).length + queues.filter((q) => isLive(q.status)).length],
              ["Security", secrets.filter((s) => isLive(s.status)).length],
            ].map(([label, value]) => (
              <div key={label} className="rounded-lg border border-border bg-muted/30 p-3">
                <div className="text-xs text-muted-foreground">{label}</div>
                <div className="mt-1 text-xl font-semibold tabular-nums">{value}</div>
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
