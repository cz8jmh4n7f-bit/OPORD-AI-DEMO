"use client";

import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { LogIn, LogOut, Menu, X } from "lucide-react";
import { Logo } from "@/components/logo";
import { cn } from "@/lib/utils";
import { isActive, sectionsFor } from "./nav";
import { useIdentity } from "@/lib/use-identity";

// MobileNav is the <768px navigation: a hamburger that opens a slide-over drawer
// with the same links + identity as the desktop sidebar. Hidden at md+ (the
// sidebar takes over). Closes on link tap, overlay click, or Escape.
export function MobileNav() {
  const pathname = usePathname();
  const { me, hasKey, logout } = useIdentity();
  const sections = sectionsFor();
  const [open, setOpen] = useState(false);

  // While open: close on Escape and lock body scroll. (Nav links close the
  // drawer via their own onClick, so no route-change effect is needed.)
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    document.addEventListener("keydown", onKey);
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = "";
    };
  }, [open]);

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        aria-label="Open navigation"
        className="inline-flex size-9 items-center justify-center rounded-lg text-foreground hover:bg-muted md:hidden"
      >
        <Menu className="size-5" />
      </button>

      {open &&
        createPortal(
        <div className="fixed inset-0 z-50 md:hidden" role="dialog" aria-modal="true" aria-label="Navigation">
          <div className="absolute inset-0 bg-black/50" onClick={() => setOpen(false)} />
          <aside className="absolute inset-y-0 left-0 flex w-72 max-w-[82%] flex-col bg-sidebar text-sidebar-foreground shadow-xl">
            <div className="flex h-16 items-center justify-between border-b border-white/10 px-5">
              <Logo />
              <button
                type="button"
                onClick={() => setOpen(false)}
                aria-label="Close navigation"
                className="inline-flex size-9 items-center justify-center rounded-lg text-sidebar-muted hover:bg-white/5 hover:text-white"
              >
                <X className="size-5" />
              </button>
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
                        onClick={() => setOpen(false)}
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
                      onClick={() => {
                        setOpen(false);
                        logout();
                      }}
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
                  onClick={() => setOpen(false)}
                  className="flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-sidebar-foreground transition-colors hover:bg-white/5 hover:text-white"
                >
                  <LogIn className="size-5 shrink-0 text-sidebar-muted" />
                  Sign in
                </Link>
              )}
            </div>
          </aside>
        </div>,
          document.body,
        )}
    </>
  );
}
