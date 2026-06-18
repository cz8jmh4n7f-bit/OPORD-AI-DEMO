"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Plus } from "lucide-react";
import { ThemeToggle } from "@/components/layout/theme-toggle";
import { MobileNav } from "@/components/layout/mobile-nav";
import { isActive, navItems, navSections } from "@/components/layout/nav";

// Breadcrumb for the current route: the section label + the active nav item's
// label (the longest href that matches, so /ai/access-review wins over /ai).
function useBreadcrumb() {
  const pathname = usePathname();
  const match = navItems
    .filter((i) => isActive(pathname, i.href))
    .sort((a, b) => b.href.length - a.href.length)[0];
  return { section: navSections[0]?.title ?? "OPORD", page: match?.label };
}

export function Topbar() {
  const { section, page } = useBreadcrumb();

  return (
    <header className="sticky top-0 z-20 flex h-11 items-center justify-between gap-4 border-b border-border bg-background px-4 md:px-6">
      <div className="flex min-w-0 items-center gap-3">
        <MobileNav />
        <nav aria-label="Breadcrumb" className="hidden min-w-0 items-center gap-2 text-[13px] md:flex">
          <span className="text-muted-foreground">{section}</span>
          {page && (
            <>
              <span className="text-faint">/</span>
              <span className="truncate text-foreground">{page}</span>
            </>
          )}
        </nav>
      </div>

      <div className="flex items-center gap-2">
        <Link
          href="/ai/catalog"
          className="inline-flex items-center gap-1 rounded-md bg-primary px-3 py-1.5 text-[13px] font-medium text-primary-foreground transition-colors hover:bg-accent-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-background"
        >
          <Plus className="size-3.5" />
          Request AI access
        </Link>
        <ThemeToggle />
        <div className="grid size-7 place-items-center rounded-full bg-surface-3 text-[11px] font-medium text-muted-foreground">
          VV
        </div>
      </div>
    </header>
  );
}
