"use client";

import { useEffect, useRef, useState } from "react";

// Parse a display value ("0", "1", "$0.00", "$1,234.50") into its prefix,
// numeric target, decimal places, and suffix so we can animate the number while
// preserving formatting.
function parse(value: string | number) {
  const s = String(value);
  const m = s.match(/^(\D*?)(-?[\d,]*\.?\d+)(\D*)$/);
  if (!m) return null;
  const num = m[2].replace(/,/g, "");
  const dot = num.indexOf(".");
  const decimals = dot === -1 ? 0 : num.length - dot - 1;
  const target = parseFloat(num);
  if (!Number.isFinite(target)) return null;
  return { prefix: m[1], suffix: m[3], decimals, target };
}

function fmt(n: number, decimals: number, prefix: string, suffix: string) {
  return prefix + n.toFixed(decimals) + suffix;
}

// CountUp animates a numeric value from 0 to its target on mount (easeOut,
// ~600ms, requestAnimationFrame). It renders the final value on the server and
// the first client paint (so there is no hydration mismatch), then runs the
// animation in an effect. Zero values and reduced-motion skip the animation.
export function CountUp({ value, duration = 600 }: { value: string | number; duration?: number }) {
  const parsed = parse(value);
  const final = parsed ? fmt(parsed.target, parsed.decimals, parsed.prefix, parsed.suffix) : String(value);
  const [display, setDisplay] = useState(final);
  const raf = useRef(0);

  useEffect(() => {
    // `display` already holds the final value (SSR-safe), so a zero target or a
    // reduced-motion preference needs no work. Otherwise the rAF callback drives
    // the count-up from 0 to target; all setState happens inside that callback.
    if (!parsed || parsed.target === 0) return;
    if (window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches) return;
    const start = performance.now();
    const tick = (now: number) => {
      const t = Math.min(1, (now - start) / duration);
      const eased = 1 - Math.pow(1 - t, 3); // easeOutCubic
      setDisplay(fmt(parsed.target * eased, parsed.decimals, parsed.prefix, parsed.suffix));
      if (t < 1) raf.current = requestAnimationFrame(tick);
    };
    raf.current = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf.current);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value, duration]);

  return <span suppressHydrationWarning>{display}</span>;
}
