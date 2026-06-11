import type { Metadata } from "next";
import Link from "next/link";
import {
  ArrowRight,
  Boxes,
  Building2,
  Star,
  Layers,
  PlugZap,
  Rocket,
  Server,
  ShieldCheck,
  SquareStack,
  Workflow,
} from "lucide-react";
import { LogoMark } from "@/components/logo";
import { button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const GITHUB = "https://github.com/cz8jmh4n7f-bit/OPORD-AI-DEMO";

export const metadata: Metadata = {
  title: "OPORD - Environment provisioning for the post-VMware era",
  description:
    "One open-source, self-hosted control plane to provision and run complete environments - Kubernetes, VMs, databases - on Proxmox, vSphere, or AWS. Declarative. Multi-backend.",
};

const features = [
  {
    icon: Layers,
    title: "Declarative blueprints",
    body: "Pick “HA Kubernetes cluster” or “web app and database” and get the whole stack - not a pile of tickets.",
  },
  {
    icon: Boxes,
    title: "Any backend, one interface",
    body: "Proxmox, VMware vSphere, AWS today. Hetzner, OVH and bare metal on the roadmap. Same API, CLI and UI.",
  },
  {
    icon: ShieldCheck,
    title: "Self-hosted by design",
    body: "You run the control plane; credentials never leave your network. Secrets resolve from your own Vault/OpenBao.",
  },
  {
    icon: Workflow,
    title: "Durable day-2 lifecycle",
    body: "Async jobs that survive restarts, drift detection, scale and destroy - built for environments that live, not 2-hour previews.",
  },
];

const audiences = [
  { icon: Server, title: "Teams leaving VMware", body: "Keep the self-service experience your engineers expect; drop the Broadcom license bill." },
  { icon: Building2, title: "MSPs & outsourcers", body: "White-label, multi-tenant. Onboard a new client and stamp identical environments in hours, not weeks." },
  { icon: ShieldCheck, title: "Regulated / on-prem", body: "A cloud-like experience without anything leaving your perimeter. RBAC, audit, and an open core." },
];

const steps = [
  { icon: PlugZap, title: "Connect a provider", body: "Register Proxmox, vSphere or AWS. Credentials stay in your network." },
  { icon: SquareStack, title: "Choose what to provision", body: "Pick a blueprint and parameters in the catalog - VM, cluster, database, or a full environment." },
  { icon: Rocket, title: "OPORD provisions and tracks it", body: "It runs OpenTofu and Ansible under the hood, then lets you scale or destroy from the UI." },
];

const backends = ["Proxmox", "VMware vSphere", "AWS"];

export default function LandingPage() {
  return (
    <div className="min-h-screen bg-background text-foreground">
      {/* Nav */}
      <header className="sticky top-0 z-30 border-b border-border bg-background/80 backdrop-blur">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-4 sm:px-6">
          <Link href="/landing" className="flex items-center gap-2.5">
            <LogoMark className="h-7 text-foreground" />
            <span className="text-lg font-bold tracking-tight">OPORD</span>
          </Link>
          <nav className="flex items-center gap-2">
            <a
              href={GITHUB}
              target="_blank"
              rel="noreferrer"
              className={cn(button({ variant: "ghost", size: "sm" }))}
            >
              <Star className="size-4" />
              <span className="hidden sm:inline">GitHub</span>
            </a>
            <Link href="/" className={cn(button({ size: "sm" }))}>
              Open console
              <ArrowRight className="size-4" />
            </Link>
          </nav>
        </div>
      </header>

      {/* Hero */}
      <section className="relative overflow-hidden border-b border-border">
        <div className="pointer-events-none absolute inset-0 bg-gradient-to-b from-navy/10 via-transparent to-transparent" />
        <div className="relative mx-auto max-w-6xl px-4 py-20 text-center sm:px-6 sm:py-28">
          <span className="inline-flex items-center gap-2 rounded-full border border-border bg-card px-3 py-1 text-xs font-medium text-muted-foreground">
            <span className="size-1.5 rounded-full bg-primary" />
            Open-source · self-hosted · alpha
          </span>
          <h1 className="mx-auto mt-6 max-w-3xl text-4xl font-bold tracking-tight sm:text-5xl">
            Environment provisioning for the <span className="text-primary">post-VMware</span> era
          </h1>
          <p className="mx-auto mt-5 max-w-2xl text-lg text-muted-foreground">
            One control plane to provision and run complete environments - Kubernetes clusters, VMs,
            databases, networks - on Proxmox, vSphere, or AWS. Declarative. Self-hosted. Open source.
          </p>
          <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
            <Link href="/" className={cn(button({ size: "md" }), "px-5")}>
              Try the alpha
              <ArrowRight className="size-4" />
            </Link>
            <a href={GITHUB} target="_blank" rel="noreferrer" className={cn(button({ variant: "outline", size: "md" }), "px-5")}>
              <Star className="size-4" />
              Star on GitHub
            </a>
          </div>
          <div className="mt-10 flex flex-wrap items-center justify-center gap-2 text-sm">
            <span className="text-muted-foreground">Runs on</span>
            {backends.map((b) => (
              <span key={b} className="rounded-md border border-border bg-card px-2.5 py-1 font-medium">
                {b}
              </span>
            ))}
            <span className="text-muted-foreground">· Hetzner, OVH, bare metal on the roadmap</span>
          </div>
        </div>
      </section>

      {/* Problem */}
      <section className="mx-auto max-w-3xl px-4 py-16 text-center sm:px-6">
        <h2 className="text-2xl font-semibold tracking-tight">The escape from VMware costs you the experience</h2>
        <p className="mt-4 text-muted-foreground">
          Broadcom turned VMware into a line item you can&apos;t defend. Proxmox is the obvious escape -
          but the moment you leave vSphere, you lose the self-service your engineers expected. Suddenly
          every new environment is a ticket, a runbook, and a week.
        </p>
      </section>

      {/* Solution / features */}
      <section className="border-y border-border bg-muted/40">
        <div className="mx-auto max-w-6xl px-4 py-16 sm:px-6">
          <div className="mx-auto max-w-2xl text-center">
            <h2 className="text-2xl font-semibold tracking-tight">The control plane for your environments</h2>
            <p className="mt-3 text-muted-foreground">
              Describe what you want once; OPORD provisions the whole thing and manages its lifecycle -
              create, scale, destroy - through one API, CLI, and UI. Same experience, your choice of backend.
            </p>
          </div>
          <div className="mt-10 grid grid-cols-1 gap-4 sm:grid-cols-2">
            {features.map((f) => (
              <div key={f.title} className="rounded-xl border border-border bg-card p-5 shadow-sm">
                <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
                  <f.icon className="size-5" />
                </div>
                <h3 className="mt-4 text-base font-semibold">{f.title}</h3>
                <p className="mt-1.5 text-sm text-muted-foreground">{f.body}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Audiences */}
      <section className="mx-auto max-w-6xl px-4 py-16 sm:px-6">
        <h2 className="text-center text-2xl font-semibold tracking-tight">Who it&apos;s for</h2>
        <div className="mt-10 grid grid-cols-1 gap-4 md:grid-cols-3">
          {audiences.map((a) => (
            <div key={a.title} className="rounded-xl border border-border bg-card p-5 shadow-sm">
              <a.icon className="size-6 text-primary" />
              <h3 className="mt-3 text-base font-semibold">{a.title}</h3>
              <p className="mt-1.5 text-sm text-muted-foreground">{a.body}</p>
            </div>
          ))}
        </div>
      </section>

      {/* How it works */}
      <section className="border-t border-border bg-muted/40">
        <div className="mx-auto max-w-6xl px-4 py-16 sm:px-6">
          <h2 className="text-center text-2xl font-semibold tracking-tight">How it works</h2>
          <div className="mt-10 grid grid-cols-1 gap-6 md:grid-cols-3">
            {steps.map((s, i) => (
              <div key={s.title} className="relative rounded-xl border border-border bg-card p-5 shadow-sm">
                <span className="absolute right-4 top-4 text-2xl font-bold tabular-nums text-muted/60">{i + 1}</span>
                <s.icon className="size-6 text-primary" />
                <h3 className="mt-3 text-base font-semibold">{s.title}</h3>
                <p className="mt-1.5 text-sm text-muted-foreground">{s.body}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Pricing */}
      <section className="mx-auto max-w-3xl px-4 py-16 text-center sm:px-6">
        <h2 className="text-2xl font-semibold tracking-tight">Open core</h2>
        <p className="mt-4 text-muted-foreground">
          Self-host the core for free, forever. Paid tiers add multi-tenancy, SSO, and support -
          exact tiers shaped with our first design partners.
        </p>
      </section>

      {/* CTA */}
      <section className="border-t border-border">
        <div className="mx-auto max-w-4xl px-4 py-16 text-center sm:px-6">
          <h2 className="text-3xl font-bold tracking-tight">Be an alpha design partner</h2>
          <p className="mx-auto mt-4 max-w-xl text-muted-foreground">
            Shape the roadmap and get hands-on help standing up your first environments.
          </p>
          <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
            <Link href="/" className={cn(button({ size: "md" }), "px-5")}>
              Open the console
              <ArrowRight className="size-4" />
            </Link>
            <a href={GITHUB} target="_blank" rel="noreferrer" className={cn(button({ variant: "outline", size: "md" }), "px-5")}>
              <Star className="size-4" />
              Star on GitHub
            </a>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-border">
        <div className="mx-auto flex max-w-6xl flex-col items-center justify-between gap-4 px-4 py-8 text-sm text-muted-foreground sm:flex-row sm:px-6">
          <div className="flex items-center gap-2">
            <LogoMark className="h-5 text-foreground" />
            <span className="font-semibold text-foreground">OPORD</span>
            <span>· Declarative infra ops</span>
          </div>
          <div className="flex items-center gap-4">
            <a href={GITHUB} target="_blank" rel="noreferrer" className="hover:text-foreground">
              GitHub
            </a>
            <Link href="/" className="hover:text-foreground">
              Console
            </Link>
          </div>
        </div>
      </footer>
    </div>
  );
}
