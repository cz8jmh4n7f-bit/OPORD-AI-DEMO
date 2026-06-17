import { cn } from "@/lib/utils";

// Double-chevron mark echoing the OPORD logo. The first chevron uses
// currentColor; the second is always brand orange.
export function LogoMark({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 46 28"
      fill="none"
      aria-hidden="true"
      className={cn("h-7 w-auto", className)}
    >
      <path d="M0 1 H12 L24 14 L12 27 H0 L12 14 Z" fill="currentColor" />
      <path d="M19 1 H31 L43 14 L31 27 H19 L31 14 Z" className="fill-primary" />
    </svg>
  );
}

export function Logo({ className }: { className?: string }) {
  return (
    <div className={cn("flex items-center gap-2.5", className)}>
      <LogoMark className="h-6 text-white" />
      <span className="text-lg font-bold tracking-tight text-white">OPORD</span>
    </div>
  );
}
