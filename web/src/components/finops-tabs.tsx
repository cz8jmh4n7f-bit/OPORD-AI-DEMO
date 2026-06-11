"use client";

import { useState } from "react";
import type { ReactNode } from "react";

export type FinopsTab = { id: string; label: string; content: ReactNode };

/**
 * Client-side tabbed container for the FinOps page. Sections are rendered on the
 * server and passed in as `content`; this component only toggles which panel is
 * visible, so switching tabs is instant (no refetch) and the long page becomes a
 * few screenfuls instead of one endless scroll.
 */
export function FinopsTabs({ tabs }: { tabs: FinopsTab[] }) {
  const [active, setActive] = useState(tabs[0]?.id ?? "");

  return (
    <div className="space-y-6">
      <div role="tablist" aria-label="FinOps sections" className="flex flex-wrap gap-1 border-b border-border">
        {tabs.map((t) => {
          const on = t.id === active;
          return (
            <button
              key={t.id}
              type="button"
              role="tab"
              aria-selected={on}
              onClick={() => setActive(t.id)}
              className={`-mb-px rounded-t-md border-b-2 px-4 py-2 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                on
                  ? "border-primary text-foreground"
                  : "border-transparent text-muted-foreground hover:text-foreground"
              }`}
            >
              {t.label}
            </button>
          );
        })}
      </div>

      {tabs.map((t) => (
        <div key={t.id} role="tabpanel" hidden={t.id !== active} className="space-y-6">
          {t.content}
        </div>
      ))}
    </div>
  );
}
