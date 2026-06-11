"use client";

import Link from "next/link";
import { Plus } from "lucide-react";
import { LogoMark } from "@/components/logo";
import { ThemeToggle } from "@/components/layout/theme-toggle";
import { AINeonSign } from "@/components/layout/ai-sign";
import { MobileNav } from "@/components/layout/mobile-nav";
import { button } from "@/components/ui/button";
import { useAIMode } from "@/lib/ai-mode";
import { useIdentity } from "@/lib/use-identity";

// initials derives a 1-2 letter avatar label from an email local-part
// (admin@opord.local -> "AD", viewer@opord.local -> "VI", dev -> "DE").
function initials(email: string): string {
  const local = (email.split("@")[0] || email).trim();
  const parts = local.split(/[._+-]+/).filter(Boolean);
  const label = parts.length >= 2 ? parts[0][0] + parts[1][0] : local.slice(0, 2);
  return label.toUpperCase() || "?";
}

export function Topbar() {
  const ai = useAIMode();
  const { me } = useIdentity();
  return (
    <header className="sticky top-0 z-20 flex h-16 items-center justify-between gap-4 border-b border-border bg-card/80 px-4 backdrop-blur md:px-6">
      <div className="flex items-center gap-3">
        <MobileNav />
        <div className="flex items-center gap-2 text-foreground md:hidden">
          <LogoMark className="h-6" />
          <span className="text-base font-bold">OPORD</span>
        </div>
        <AINeonSign />
      </div>

      <div className="flex items-center gap-2">
        <Link href={ai ? "/ai/catalog" : "/catalog"} className={button({ size: "sm" })}>
          <Plus className="size-4" />
          {ai ? "Request AI access" : "Request service"}
        </Link>
        <ThemeToggle />
        {me ? (
          <div
            className="grid size-8 place-items-center rounded-full bg-navy text-xs font-semibold text-white"
            title={`${me.email} - ${me.role}`}
          >
            {initials(me.email)}
          </div>
        ) : (
          <Link href="/login" className={button({ variant: "outline", size: "sm" })}>
            Sign in
          </Link>
        )}
      </div>
    </header>
  );
}
