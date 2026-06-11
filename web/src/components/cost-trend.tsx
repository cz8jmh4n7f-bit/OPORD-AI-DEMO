// CostTrend renders a responsive, dependency-free inline-SVG area+line chart of
// daily cloud spend. It uses a fixed internal coordinate space (0..W, 0..H) and
// preserveAspectRatio="none" so it stretches to its container width while the
// vertical scale stays meaningful. Stroke/fill use the app's primary color token.
// Plain (server-renderable) - no client interactivity.

function usd(n: number): string {
  return "$" + n.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

export function CostTrend({ points }: { points: { date: string; usd: number }[] }) {
  // Need at least two points to draw a meaningful line.
  if (points.length < 2) {
    return (
      <div className="flex h-40 items-center justify-center rounded-lg border border-dashed border-border bg-muted/20">
        <p className="text-sm text-muted-foreground">Not enough data to chart a daily trend yet.</p>
      </div>
    );
  }

  // Internal coordinate space. preserveAspectRatio="none" maps W to the container
  // width; H stays the drawing height. Pad the top so the line never clips.
  const W = 600;
  const H = 160;
  const padTop = 8;
  const padBottom = 0;

  const max = Math.max(...points.map((p) => p.usd), 0);
  const denom = max > 0 ? max : 1;
  const stepX = points.length > 1 ? W / (points.length - 1) : W;
  const yFor = (v: number) => padTop + (1 - v / denom) * (H - padTop - padBottom);

  const coords = points.map((p, i) => ({ x: i * stepX, y: yFor(p.usd) }));
  const linePath = coords.map((c, i) => `${i === 0 ? "M" : "L"}${c.x.toFixed(2)},${c.y.toFixed(2)}`).join(" ");
  // Close the area down to the baseline for the fill.
  const areaPath = `${linePath} L${W},${H} L0,${H} Z`;
  const baselineY = yFor(0);

  const first = points[0];
  const last = points[points.length - 1];

  return (
    <div className="space-y-2">
      <svg
        viewBox={`0 0 ${W} ${H}`}
        preserveAspectRatio="none"
        className="h-40 w-full"
        role="img"
        aria-label={`Daily spend from ${first.date} to ${last.date}, peak ${usd(max)}`}
      >
        {/* subtle zero baseline */}
        <line x1="0" y1={baselineY} x2={W} y2={baselineY} className="stroke-border" strokeWidth="1" vectorEffect="non-scaling-stroke" />
        {/* filled area under the line */}
        <path d={areaPath} className="fill-primary/15" />
        {/* the spend line */}
        <path
          d={linePath}
          className="fill-none stroke-primary"
          strokeWidth="2"
          strokeLinejoin="round"
          strokeLinecap="round"
          vectorEffect="non-scaling-stroke"
        />
      </svg>
      <div className="flex items-center justify-between text-[11px] text-muted-foreground tabular-nums">
        <span>{first.date}</span>
        <span>peak {usd(max)}</span>
        <span>{last.date}</span>
      </div>
    </div>
  );
}
