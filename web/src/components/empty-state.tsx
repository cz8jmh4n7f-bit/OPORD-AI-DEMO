import Link from "next/link";
import type { LucideIcon } from "lucide-react";
import { button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

// EmptyState is the friendly "nothing here yet" placeholder for list pages: an
// icon, a title, a short description, and an optional primary action.
export function EmptyState({
  icon: Icon,
  title,
  description,
  action,
}: {
  icon: LucideIcon;
  title: string;
  description?: string;
  action?: { href: string; label: string };
}) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 px-6 py-16 text-center">
      <div className="grid size-12 place-items-center rounded-xl bg-muted text-muted-foreground">
        <Icon className="size-6" />
      </div>
      <div className="space-y-1">
        <p className="text-sm font-semibold text-foreground">{title}</p>
        {description && (
          <p className="mx-auto max-w-sm text-sm text-muted-foreground">{description}</p>
        )}
      </div>
      {action && (
        <Link href={action.href} className={cn(button({ size: "sm" }), "mt-1")}>
          {action.label}
        </Link>
      )}
    </div>
  );
}
