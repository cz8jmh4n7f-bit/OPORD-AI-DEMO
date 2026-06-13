"use client";

import type { ReactNode } from "react";
import { usePathname } from "next/navigation";
import { TriangleAlert } from "lucide-react";
import { Sidebar } from "@/components/layout/sidebar";
import { Topbar } from "@/components/layout/topbar";

// Routes rendered WITHOUT the console shell (sidebar/topbar) - e.g. the public
// marketing landing. Everything else gets the full app shell. Both workspaces
// (infrastructure + AI governance) are first-class - the nav follows the route.
const bareRoutes = ["/landing"];

export function LayoutShell({ apiOk, children }: { apiOk: boolean; children: ReactNode }) {
  const pathname = usePathname();
  const bare = bareRoutes.some((p) => pathname === p || pathname.startsWith(`${p}/`));

  if (bare) return <>{children}</>;

  return (
    <div className="flex min-h-screen">
      <a
        href="#main"
        className="sr-only rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground focus:not-sr-only focus:absolute focus:left-4 focus:top-4 focus:z-[100] focus:shadow-lg"
      >
        Skip to content
      </a>
      <Sidebar />
      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar />
        {!apiOk && (
          <div
            role="alert"
            className="flex items-center gap-2 border-b border-warning/30 bg-warning/10 px-4 py-2 text-sm text-warning md:px-6 lg:px-8"
          >
            <TriangleAlert className="size-4 shrink-0" />
            <span>
              Can&apos;t reach the OPORD API - data may be unavailable. Start{" "}
              <code className="rounded bg-warning/15 px-1 py-0.5 text-xs">opord-api</code> and reload.
            </span>
          </div>
        )}
        <main id="main" tabIndex={-1} className="flex-1 p-4 outline-none md:p-6 lg:p-8">
          {children}
        </main>
      </div>
    </div>
  );
}
