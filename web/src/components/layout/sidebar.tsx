"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { LogIn, LogOut } from "lucide-react";
import { Logo } from "@/components/logo";
import { cn } from "@/lib/utils";
import { isActive, sectionsFor } from "./nav";
import { useIdentity } from "@/lib/use-identity";

export function Sidebar() {
  const pathname = usePathname();
  const { me, hasKey, logout } = useIdentity();
  const sections = sectionsFor();

  return (
    <aside className="hidden w-64 shrink-0 flex-col bg-sidebar text-sidebar-foreground md:sticky md:top-0 md:flex md:h-screen">
      <div className="flex h-16 items-center border-b border-white/10 px-5">
        <Logo />
      </div>

      <nav className="flex-1 space-y-4 overflow-y-auto p-3">
        {sections.map((section) => (
          <div key={section.title} className="space-y-1">
            <div className="px-3 pb-0.5 text-[11px] font-semibold uppercase tracking-wider text-sidebar-muted">
              {section.title}
            </div>
            {section.items.map(({ href, label, icon: Icon }) => {
              const active = isActive(pathname, href);
              return (
                <Link
                  key={href}
                  href={href}
                  aria-current={active ? "page" : undefined}
                  className={cn(
                    "group flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                    active
                      ? "bg-white/10 text-white"
                      : "text-sidebar-foreground hover:bg-white/5 hover:text-white",
                  )}
                >
                  <Icon aria-hidden className={cn("size-5 shrink-0", active ? "text-primary" : "text-sidebar-muted")} />
                  {label}
                </Link>
              );
            })}
          </div>
        ))}
      </nav>

      <div className="border-t border-white/10 p-3">
        {me ? (
          <div className="space-y-1">
            <div className="px-3 py-1">
              <div className="truncate text-sm font-medium text-white">{me.email}</div>
              <div className="text-xs text-sidebar-muted">
                {me.role} · {me.tenant}
              </div>
            </div>
            {hasKey && (
              <button
                type="button"
                onClick={logout}
                className="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-sidebar-foreground transition-colors hover:bg-white/5 hover:text-white"
              >
                <LogOut className="size-5 shrink-0 text-sidebar-muted" />
                Sign out
              </button>
            )}
          </div>
        ) : (
          <Link
            href="/login"
            className="flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-sidebar-foreground transition-colors hover:bg-white/5 hover:text-white"
          >
            <LogIn className="size-5 shrink-0 text-sidebar-muted" />
            Sign in
          </Link>
        )}
      </div>
    </aside>
  );
}
