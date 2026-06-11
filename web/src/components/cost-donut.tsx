// Dependency-free SVG donut for cost composition (spend by account / service).
// Zero-cost entries drop out; tiny slices beyond `maxSlices` fold into "Other",
// so the long $0.00 service list collapses into a readable ring + legend.

const PALETTE = [
  "#f97316", // orange-500 (brand)
  "#fbbf24", // amber-400
  "#38bdf8", // sky-400
  "#34d399", // emerald-400
  "#a78bfa", // violet-400
  "#fb7185", // rose-400
  "#22d3ee", // cyan-400
  "#94a3b8", // slate-400 (Other)
];

function usd(n: number): string {
  return "$" + n.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

export function CostDonut({
  items,
  empty,
  maxSlices = 7,
}: {
  items: { name: string; usd: number }[];
  empty: string;
  maxSlices?: number;
}) {
  const positive = items.filter((i) => i.usd > 0).sort((a, b) => b.usd - a.usd);

  let slices = positive;
  if (positive.length > maxSlices) {
    const top = positive.slice(0, maxSlices);
    const otherUsd = positive.slice(maxSlices).reduce((s, i) => s + i.usd, 0);
    slices = otherUsd > 0 ? [...top, { name: "Other", usd: otherUsd }] : top;
  }

  const total = slices.reduce((s, i) => s + i.usd, 0);
  if (total <= 0) {
    return <p className="text-sm text-muted-foreground">{empty}</p>;
  }

  const size = 180;
  const stroke = 26;
  const r = (size - stroke) / 2;
  const c = 2 * Math.PI * r;

  const fracs = slices.map((s) => s.usd / total);
  const arcs = slices.map((s, i) => {
    const before = fracs.slice(0, i).reduce((a, f) => a + f, 0); // cumulative fraction before this slice
    return {
      name: s.name,
      usd: s.usd,
      color: PALETTE[i % PALETTE.length],
      dash: fracs[i] * c,
      offset: -before * c,
      pct: fracs[i] * 100,
    };
  });

  // Integer percentages that sum to exactly 100 (largest slice absorbs the rounding drift).
  const intPct = arcs.map((a) => Math.round(a.pct));
  const maxIdx = arcs.reduce((mi, a, i, arr) => (a.pct > arr[mi].pct ? i : mi), 0);
  const drift = 100 - intPct.reduce((sum, p) => sum + p, 0);
  const adjPct = intPct.map((p, i) => (i === maxIdx ? p + drift : p));

  return (
    <div className="flex flex-col items-center gap-5 sm:flex-row">
      <div className="relative size-44 shrink-0">
        <svg viewBox={`0 0 ${size} ${size}`} className="size-full -rotate-90">
          <circle cx={size / 2} cy={size / 2} r={r} fill="none" className="stroke-muted" strokeWidth={stroke} />
          {arcs.map((a) => (
            <circle
              key={a.name}
              cx={size / 2}
              cy={size / 2}
              r={r}
              fill="none"
              stroke={a.color}
              strokeWidth={stroke}
              strokeDasharray={`${a.dash} ${c - a.dash}`}
              strokeDashoffset={a.offset}
            />
          ))}
        </svg>
        <div className="absolute inset-0 flex flex-col items-center justify-center">
          <span className="text-[10px] uppercase tracking-wide text-muted-foreground">total</span>
          <span className="text-base font-semibold tabular-nums">{usd(total)}</span>
        </div>
      </div>

      <ul className="min-w-0 flex-1 space-y-1.5 text-sm">
        {arcs.map((a, i) => (
          <li key={a.name} className="flex items-center justify-between gap-3">
            <span className="flex min-w-0 items-center gap-2">
              <span className="size-2.5 shrink-0 rounded-full" style={{ background: a.color }} aria-hidden />
              <span className="truncate">{a.name}</span>
            </span>
            <span className="shrink-0 tabular-nums text-muted-foreground">
              {usd(a.usd)} · {adjPct[i]}%
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}
