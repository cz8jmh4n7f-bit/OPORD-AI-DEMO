"use client";

import { useState, useTransition } from "react";
import { useRouter, usePathname } from "next/navigation";
import { Layers, Loader2 } from "lucide-react";
import { ProviderBrand } from "@/components/provider-brand";
import type { FinOpsCloud, ProviderType } from "@/lib/types";

function usd(n: number): string {
  return "$" + n.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

// CloudTiles is the primary FinOps scope selector: an "All clouds" tile plus one
// tile per connected cost-reporting cloud. The list is STABLE - a cloud whose cost
// query failed this load (e.g. transient Azure Cost Management) renders greyed +
// disabled with "unavailable" rather than vanishing. Selecting an available tile
// pushes ?provider=<name> (clearing the account) inside a transition: the target
// highlights + spins while the server re-queries every cloud.
export function CloudTiles({
  clouds,
  active,
  days,
}: {
  clouds: FinOpsCloud[];
  active: string;
  days: number;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const [isPending, startTransition] = useTransition();
  const [target, setTarget] = useState<string | null>(null);

  function select(name: string) {
    if (name === active) return;
    setTarget(name);
    const params = new URLSearchParams();
    if (name) params.set("provider", name);
    if (days !== 30) params.set("days", String(days));
    const qs = params.toString();
    startTransition(() => router.push(qs ? `${pathname}?${qs}` : pathname));
  }

  const total = clouds.filter((c) => c.available).reduce((sum, c) => sum + c.usd, 0);
  const tiles = [
    { type: "all", name: "", label: "All clouds", usd: total, available: true, error: undefined as string | undefined },
    ...clouds.map((c) => ({ type: c.type, name: c.name, label: c.name, usd: c.usd, available: c.available, error: c.error })),
  ];

  const effective = isPending && target !== null ? target : active;

  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4" aria-busy={isPending}>
      {tiles.map((t) => {
        const isAll = t.type === "all";
        const on = isAll ? !effective : t.name === effective;
        const loading = isPending && (target === t.name || (isAll && target === ""));
        return (
          <button
            key={t.name || "all"}
            type="button"
            onClick={() => t.available && select(t.name)}
            disabled={isPending || !t.available}
            aria-pressed={on}
            title={t.available ? undefined : `Cost data unavailable: ${t.error ?? "temporary error"}`}
            className={`flex items-center gap-3 rounded-xl border p-4 text-left transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
              on
                ? "border-primary bg-primary/10 ring-2 ring-primary/40 shadow-md"
                : "border-border bg-card hover:border-primary/40 hover:bg-muted"
            } ${!t.available ? "cursor-not-allowed opacity-45" : ""} ${
              isPending && !loading && t.available ? "opacity-50" : ""
            } ${isPending && t.available ? "cursor-wait" : ""}`}
          >
            <span className="grid size-9 shrink-0 place-items-center rounded-lg bg-muted">
              {loading ? (
                <Loader2 className="size-5 animate-spin text-primary" />
              ) : isAll ? (
                <Layers className="size-5 text-primary" />
              ) : (
                <ProviderBrand type={t.type as ProviderType} className={`size-5 ${t.available ? "" : "grayscale"}`} iconOnly />
              )}
            </span>
            <span className="min-w-0">
              <span className="block truncate text-sm font-medium">{t.label}</span>
              <span className="block text-xs tabular-nums text-muted-foreground">
                {loading ? "loading…" : t.available ? usd(t.usd) : "unavailable"}
              </span>
            </span>
          </button>
        );
      })}
    </div>
  );
}
