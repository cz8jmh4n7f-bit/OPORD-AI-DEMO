// The AI workspace is ROUTE-scoped: any /ai/* page is "in" it, everything else
// is the infrastructure workspace. Both surfaces are first-class (one product,
// two catalogs) - the topbar AI sign is a navigation switch between them, not a
// feature gate. Deriving the state from the URL keeps the nav, the sign, and the
// page content coherent by construction (no persisted mode to drift out of sync).
"use client";

import { usePathname } from "next/navigation";

/** useAIMode reports whether the current route is inside the AI workspace. */
export function useAIMode(): boolean {
  return (usePathname() ?? "").startsWith("/ai");
}
