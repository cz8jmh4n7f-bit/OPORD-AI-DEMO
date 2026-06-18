import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

// A zero value reads dimmer than a real one, so an empty console doesn't shout
// numbers. Covers 0, "0", and money-zeros like "$0.00".
function isZeroValue(value: string | number): boolean {
  if (typeof value === "number") return value === 0;
  return /^\$?0(\.0+)?$/.test(value.trim());
}

// StatCard — the single stat standard: uppercase label + faint corner icon, a
// large tabular value, a faint subtitle. Flat surface, hairline border that
// strengthens on hover (no shadow, no colored icon circle). The legacy `accent`
// prop is accepted but ignored (kept so existing call sites compile).
export function StatCard({
  icon: Icon,
  label,
  value,
  hint,
}: {
  icon?: LucideIcon;
  label: string;
  value: string | number;
  hint?: string;
  accent?: string;
}) {
  const zero = isZeroValue(value);
  return (
    <div className="rounded-lg border border-border bg-surface-2 p-4 transition-colors hover:border-border-strong">
      <div className="flex items-start justify-between gap-2">
        <span className="text-[11px] font-medium uppercase tracking-[0.06em] text-muted-foreground">
          {label}
        </span>
        {Icon && <Icon aria-hidden className="size-4 shrink-0 text-faint" />}
      </div>
      <div
        className={cn(
          "mt-3 text-[28px] font-semibold leading-none tabular-nums",
          zero ? "text-muted-foreground" : "text-foreground",
        )}
      >
        {value}
      </div>
      {hint && <div className="mt-1.5 text-xs text-faint">{hint}</div>}
    </div>
  );
}
