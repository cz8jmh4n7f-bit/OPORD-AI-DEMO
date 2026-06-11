import { PageHeader } from "@/components/page-header";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { JobStatusBadge } from "@/components/status-badge";
import { fetchJobs, fetchQueue } from "@/lib/api";
import type { Job, QueueJob } from "@/lib/types";
import { cn, formatDate } from "@/lib/utils";

function duration(j: Job): string {
  let secs = j.durationSecs ?? null;
  if (secs == null && j.startedAt && j.finishedAt) {
    secs = Math.round((new Date(j.finishedAt).getTime() - new Date(j.startedAt).getTime()) / 1000);
  }
  if (secs == null) return "-";
  const m = Math.floor(secs / 60);
  const s = secs % 60;
  return m > 0 ? `${m}m ${s}s` : `${s}s`;
}

// River queue states -> badge color.
const queueStateClass: Record<string, string> = {
  available: "bg-info/10 text-info",
  pending: "bg-info/10 text-info",
  scheduled: "bg-info/10 text-info",
  running: "bg-warning/10 text-warning",
  retryable: "bg-warning/10 text-warning",
  completed: "bg-success/10 text-success",
  cancelled: "bg-muted text-muted-foreground",
  discarded: "bg-danger/10 text-danger",
};

function QueueStateBadge({ state }: { state: string }) {
  return (
    <span className={cn("rounded-md px-2 py-0.5 text-xs font-medium", queueStateClass[state] ?? "bg-muted text-muted-foreground")}>
      {state}
    </span>
  );
}

export const metadata = { title: "Jobs" };

export default async function JobsPage() {
  const [jobs, queue] = await Promise.all([fetchJobs(), fetchQueue()]);

  return (
    <div className="space-y-6">
      <PageHeader title="Jobs" description="Durable job queue (River) and per-cluster operations." />

      <Card className="overflow-hidden p-0">
        <CardHeader className="p-5 pb-3">
          <CardTitle>
            Job queue <span className="text-muted-foreground">(River · {queue.length})</span>
          </CardTitle>
        </CardHeader>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-y border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
                <th scope="col" className="px-5 py-2.5 font-medium">ID</th>
                <th scope="col" className="px-5 py-2.5 font-medium">Kind</th>
                <th scope="col" className="px-5 py-2.5 font-medium">State</th>
                <th scope="col" className="px-5 py-2.5 font-medium">Attempt</th>
                <th scope="col" className="px-5 py-2.5 font-medium">Created</th>
                <th scope="col" className="px-5 py-2.5 font-medium">Finalized</th>
              </tr>
            </thead>
            <tbody>
              {queue.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-5 py-6 text-center text-muted-foreground">
                    Queue empty - start <span className="font-mono">opord-worker</span> and create a resource.
                  </td>
                </tr>
              )}
              {queue.map((q: QueueJob) => (
                <tr key={q.id} className="border-b border-border last:border-0 align-top hover:bg-muted/60">
                  <td className="px-5 py-2.5 font-mono text-xs text-muted-foreground">{q.id}</td>
                  <td className="px-5 py-2.5">
                    <span className="rounded-md bg-muted px-2 py-0.5 font-mono text-xs text-muted-foreground">{q.kind}</span>
                  </td>
                  <td className="px-5 py-2.5">
                    <QueueStateBadge state={q.state} />
                    {q.error && (
                      <p className="mt-1 max-w-md font-mono text-xs leading-relaxed text-danger">{q.error}</p>
                    )}
                  </td>
                  <td className="px-5 py-2.5 text-muted-foreground">
                    {q.attempt}/{q.maxAttempts}
                  </td>
                  <td className="px-5 py-2.5 text-muted-foreground">{formatDate(q.createdAt)}</td>
                  <td className="px-5 py-2.5 text-muted-foreground">{q.finalizedAt ? formatDate(q.finalizedAt) : "-"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>

      <Card className="overflow-hidden p-0">
        <CardHeader className="p-5 pb-3">
          <CardTitle>
            Cluster operations <span className="text-muted-foreground">({jobs.length})</span>
          </CardTitle>
        </CardHeader>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
                <th scope="col" className="px-5 py-3 font-medium">Cluster</th>
                <th scope="col" className="px-5 py-3 font-medium">Operation</th>
                <th scope="col" className="px-5 py-3 font-medium">Status</th>
                <th scope="col" className="px-5 py-3 font-medium">Started</th>
                <th scope="col" className="px-5 py-3 font-medium">Duration</th>
              </tr>
            </thead>
            <tbody>
              {jobs.length === 0 && (
                <tr>
                  <td colSpan={5} className="px-5 py-6 text-center text-muted-foreground">
                    No jobs yet.
                  </td>
                </tr>
              )}
              {jobs.map((j) => {
                const started = j.startedAt ?? j.createdAt ?? null;
                return (
                  <tr key={j.id} className="border-b border-border last:border-0 align-top hover:bg-muted/60">
                    <td className="px-5 py-3 font-medium">{j.cluster}</td>
                    <td className="px-5 py-3">
                      <span className="rounded-md bg-muted px-2 py-0.5 font-mono text-xs text-muted-foreground">
                        {j.operation}
                      </span>
                    </td>
                    <td className="px-5 py-3">
                      <JobStatusBadge status={j.status} />
                      {j.error && (
                        <p className="mt-1 max-w-md font-mono text-xs leading-relaxed text-danger">
                          {j.error}
                        </p>
                      )}
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{started ? formatDate(started) : "-"}</td>
                    <td className="px-5 py-3 text-muted-foreground">{duration(j)}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </Card>
    </div>
  );
}
