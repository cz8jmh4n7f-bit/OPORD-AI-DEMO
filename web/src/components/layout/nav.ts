import {
  Bot,
  BrainCircuit,
  Building2,
  CircleDollarSign,
  Cpu,
  History,
  LayoutDashboard,
  MessageSquarePlus,
  Server,
  ShieldCheck,
  SlidersHorizontal,
  Sparkles,
  Workflow,
  Zap,
  type LucideIcon,
} from "lucide-react";

export type NavItem = { href: string; label: string; icon: LucideIcon };
export type NavSection = { title: string; items: NavItem[] };

// AI service governance navigation - the single source of truth for the desktop
// sidebar and the mobile drawer.
export const navSections: NavSection[] = [
  {
    title: "AI Governance",
    items: [
      { href: "/ai/overview", label: "Overview", icon: LayoutDashboard },
      { href: "/ai/catalog", label: "AI Services", icon: Sparkles },
      { href: "/ai/requests", label: "Requests", icon: MessageSquarePlus },
      { href: "/ai/instances", label: "Access", icon: Bot },
      { href: "/ai/admin", label: "Org Admin", icon: Building2 },
      { href: "/ai/mcp", label: "Agents & MCP", icon: Workflow },
      { href: "/ai/usage", label: "Usage", icon: BrainCircuit },
      { href: "/ai/budgets", label: "Budgets", icon: CircleDollarSign },
      { href: "/ai/quotas", label: "Quotas", icon: SlidersHorizontal },
      { href: "/ai/policies", label: "Policies", icon: ShieldCheck },
      { href: "/ai/models", label: "Models", icon: Cpu },
      { href: "/ai/renewals", label: "Renewals", icon: Workflow },
      { href: "/ai/access-review", label: "Access Review", icon: ShieldCheck },
      { href: "/ai/gateway", label: "Gateway", icon: Zap },
      { href: "/ai/providers", label: "Providers", icon: Server },
      { href: "/ai/audit", label: "Audit", icon: History },
    ],
  },
];

// Flattened list for consumers that need every nav link (e.g. active-route lookup).
export const navItems: NavItem[] = navSections.flatMap((s) => s.items);

// isActive matches the dashboard exactly and other routes by prefix.
export function isActive(pathname: string, href: string): boolean {
  return href === "/" ? pathname === "/" : pathname.startsWith(href);
}

// sectionsFor returns the nav sections (AI-governance-only product).
export function sectionsFor(): NavSection[] {
  return navSections;
}
