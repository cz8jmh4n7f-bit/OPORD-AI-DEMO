"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Logo } from "@/components/logo";
import { cn } from "@/lib/utils";
import { isActive, sectionsFor } from "./nav";
import { useIdentity } from "@/lib/use-identity";

// Text-only navigation against a surface that differs from the page background
// (no border-right). The active item is marked by a 2px left accent bar and a
// brighter label, not a filled background — the command-center treatment.
export function Sidebar() {
  const pathname = usePathname();
  const { me, hasKey, logout } = useIdentity();
  const sections = sectionsFor();

  return (
    <aside className="hidden w-[200px] shrink-0 flex-col bg-sidebar md:sticky md:top-0 md:flex md:h-screen">
      <div className="flex h-11 items-center px-5">
        <Link href="/ai/overview" aria-label="OPORD home">
          <Logo />
        </Link>
      </div>

      <nav className="flex-1 space-y-5 overflow-y-auto px-3 py-4">
        {sections.map((section) => (
          <div key={section.title} className="space-y-0.5">
            <div className="px-3 pb-1.5 text-[10px] font-semibold uppercase tracking-[0.1em] text-faint">
              {section.title}
            </div>
            {section.items.map(({ href, label }) => {
              const active = isActive(pathname, href);
              return (
                <Link
                  key={href}
                  href={href}
                  aria-current={active ? "page" : undefined}
                  className={cn(
                    "block border-l-2 py-1.5 pl-3 text-[13px] transition-colors",
                    active
                      ? "border-primary font-medium text-foreground"
                      : "border-transparent text-sidebar-foreground hover:text-foreground",
                  )}
                >
                  {label}
                </Link>
              );
            })}
          </div>
        ))}
      </nav>

      <div className="px-5 py-4">
        {me ? (
          <div className="flex items-center justify-between gap-2 text-[11px] text-faint">
            <span className="truncate">
              {me.role} · {me.tenant}
            </span>
            {hasKey && (
              <button
                type="button"
                onClick={logout}
                className="shrink-0 text-faint transition-colors hover:text-foreground"
              >
                Sign out
              </button>
            )}
          </div>
        ) : (
          <Link href="/login" className="text-[11px] text-faint transition-colors hover:text-foreground">
            Sign in
          </Link>
        )}
      </div>
    </aside>
  );
}
