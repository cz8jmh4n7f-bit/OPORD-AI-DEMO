import Link from "next/link";
import type { LucideIcon } from "lucide-react";

// EmptyState — left-aligned, text-only. No illustration, no icon-in-circle, no
// filled button: emptiness is fine. An optional inline text link points to the
// next step. The `icon` prop is accepted but not rendered (kept for compat).
export function EmptyState({
  title,
  description,
  action,
}: {
  icon?: LucideIcon;
  title: string;
  description?: string;
  action?: { href: string; label: string };
}) {
  return (
    <div className="px-1 py-10">
      <p className="text-sm font-medium text-muted-foreground">{title}</p>
      {description && <p className="mt-1 max-w-prose text-sm text-faint">{description}</p>}
      {action && (
        <Link
          href={action.href}
          className="mt-3 inline-flex items-center gap-1 text-[13px] font-medium text-primary transition-colors hover:text-accent-hover"
        >
          {action.label} →
        </Link>
      )}
    </div>
  );
}
