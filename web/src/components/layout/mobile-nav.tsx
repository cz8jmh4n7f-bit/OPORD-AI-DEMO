"use client";

import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Menu, X } from "lucide-react";
import { Logo } from "@/components/logo";
import { cn } from "@/lib/utils";
import { isActive, sectionsFor } from "./nav";
import { useIdentity } from "@/lib/use-identity";

// MobileNav is the <768px navigation: a hamburger that opens a slide-over drawer
// with the same text-only links + identity as the desktop sidebar. Hidden at
// md+ (the sidebar takes over). Closes on link tap, overlay click, or Escape.
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
        className="inline-flex size-8 items-center justify-center rounded-md text-foreground hover:bg-surface-3 md:hidden"
      >
        <Menu className="size-5" />
      </button>

      {open &&
        createPortal(
          <div className="fixed inset-0 z-50 md:hidden" role="dialog" aria-modal="true" aria-label="Navigation">
            <div className="absolute inset-0 bg-black/60" onClick={() => setOpen(false)} />
            <aside className="absolute inset-y-0 left-0 flex w-64 max-w-[82%] flex-col bg-sidebar shadow-xl">
              <div className="flex h-12 items-center justify-between px-5">
                <Logo />
                <button
                  type="button"
                  onClick={() => setOpen(false)}
                  aria-label="Close navigation"
                  className="inline-flex size-8 items-center justify-center rounded-md text-muted-foreground hover:bg-surface-3 hover:text-foreground"
                >
                  <X className="size-5" />
                </button>
              </div>

              <nav className="flex-1 space-y-5 overflow-y-auto px-3 py-3">
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
                          onClick={() => setOpen(false)}
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
                        onClick={() => {
                          setOpen(false);
                          logout();
                        }}
                        className="shrink-0 text-faint transition-colors hover:text-foreground"
                      >
                        Sign out
                      </button>
                    )}
                  </div>
                ) : (
                  <Link
                    href="/login"
                    onClick={() => setOpen(false)}
                    className="text-[11px] text-faint transition-colors hover:text-foreground"
                  >
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
