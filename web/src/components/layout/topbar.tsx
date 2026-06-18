"use client";

import Link from "next/link";
import { Plus } from "lucide-react";
import { LogoMark } from "@/components/logo";
import { ThemeToggle } from "@/components/layout/theme-toggle";
import { MobileNav } from "@/components/layout/mobile-nav";
import { button } from "@/components/ui/button";

export function Topbar() {
  return (
    <header className="sticky top-0 z-20 flex h-16 items-center justify-between gap-4 border-b border-border bg-card/80 px-4 backdrop-blur md:px-6">
      <div className="flex items-center gap-3">
        <MobileNav />
        <div className="flex items-center gap-2 text-foreground md:hidden">
          <LogoMark className="h-6" />
          <span className="text-base font-bold">OPORD</span>
        </div>
      </div>

      <div className="flex items-center gap-2">
        <Link href="/ai/catalog" className={button({ size: "sm" })}>
          <Plus className="size-4" />
          Request AI access
        </Link>
        <ThemeToggle />
        <div className="grid size-8 place-items-center rounded-full bg-navy text-xs font-semibold text-white">
          VV
        </div>
      </div>
    </header>
  );
}
