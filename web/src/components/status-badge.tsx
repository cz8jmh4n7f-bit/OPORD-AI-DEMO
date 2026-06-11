import { Badge, type BadgeVariant } from "@/components/ui/badge";
import type { ClusterStatus, JobStatus } from "@/lib/types";
import { cn } from "@/lib/utils";

type Meta = { variant: BadgeVariant; label: string; pulse?: boolean };

const clusterMeta: Record<ClusterStatus, Meta> = {
  pending: { variant: "default", label: "Pending" },
  provisioning: { variant: "info", label: "Provisioning", pulse: true },
  bootstrapping: { variant: "warning", label: "Bootstrapping", pulse: true },
  ready: { variant: "success", label: "Ready" },
  degraded: { variant: "warning", label: "Degraded" },
  destroying: { variant: "warning", label: "Destroying", pulse: true },
  destroyed: { variant: "default", label: "Destroyed" },
  failed: { variant: "danger", label: "Failed" },
};

const jobMeta: Record<JobStatus, Meta> = {
  queued: { variant: "default", label: "Queued" },
  running: { variant: "info", label: "Running", pulse: true },
  succeeded: { variant: "success", label: "Succeeded" },
  failed: { variant: "danger", label: "Failed" },
  cancelled: { variant: "default", label: "Cancelled" },
};

function Dot({ pulse }: { pulse?: boolean }) {
  return (
    <span className="relative flex size-1.5">
      {pulse && (
        <span className="absolute inline-flex size-full animate-ping rounded-full bg-current opacity-60" />
      )}
      <span className="relative inline-flex size-1.5 rounded-full bg-current" />
    </span>
  );
}

export function ClusterStatusBadge({
  status,
  className,
  error,
}: {
  status: ClusterStatus;
  className?: string;
  // Finding E: when a resource failed, error carries the provision-failure
  // reason. It's shown as a native tooltip on hover and as a small muted line
  // under the badge, so the UI surfaces WHY it failed (not just "Failed").
  error?: string;
}) {
  const m = clusterMeta[status];
  const showError = status === "failed" && !!error;
  return (
    <div className="flex flex-col items-start gap-0.5">
      <Badge variant={m.variant} className={cn(className)} title={showError ? error : undefined}>
        <Dot pulse={m.pulse} />
        {m.label}
      </Badge>
      {showError && (
        <span className="max-w-xs truncate text-xs text-muted-foreground" title={error}>
          {error}
        </span>
      )}
    </div>
  );
}

export function JobStatusBadge({ status, className }: { status: JobStatus; className?: string }) {
  const m = jobMeta[status];
  return (
    <Badge variant={m.variant} className={cn(className)}>
      <Dot pulse={m.pulse} />
      {m.label}
    </Badge>
  );
}
