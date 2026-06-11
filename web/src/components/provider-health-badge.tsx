import type { ProviderHealth } from "@/lib/types";
import { cn } from "@/lib/utils";

// ProviderHealthBadge renders the last connectivity-probe result as a colored
// dot + label, with the full error message on hover and a relative timestamp.
// Presentational only (safe in a server component).
export function ProviderHealthBadge({ health }: { health?: ProviderHealth }) {
  const status = health?.status ?? "";
  const meta = {
    ok: { dot: "bg-success", label: "Healthy" },
    failed: { dot: "bg-danger", label: "Failed" },
    unsupported: { dot: "bg-muted-foreground", label: "Not supported" },
    "": { dot: "bg-border", label: "Not checked" },
  }[status];

  return (
    <div className="flex flex-col gap-0.5" title={health?.message || undefined}>
      <span className="inline-flex items-center gap-1.5 text-xs font-medium text-foreground">
        <span className={cn("size-2 shrink-0 rounded-full", meta.dot)} aria-hidden />
        {meta.label}
        {status === "ok" && health?.latencyMs ? (
          <span className="text-muted-foreground">· {health.latencyMs}ms</span>
        ) : null}
      </span>
      {health?.checkedAt && <span className="text-[11px] text-muted-foreground">{relative(health.checkedAt)}</span>}
    </div>
  );
}

// relative renders a compact "x ago" string from an ISO timestamp.
function relative(iso: string): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const s = Math.max(0, Math.round((Date.now() - then) / 1000));
  if (s < 60) return "just now";
  const m = Math.round(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.round(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.round(h / 24)}d ago`;
}
