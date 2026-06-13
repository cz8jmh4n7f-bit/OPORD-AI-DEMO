import Link from "next/link";
import { ArrowRight, CheckCircle2, LayoutGrid, PlugZap, ShieldCheck, Sparkles } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type Step = {
  icon: typeof PlugZap;
  title: string;
  body: string;
  href: string;
  label: string;
  done: boolean;
};

// GetStarted is the first-run onboarding checklist on Home: a fresh install is a
// wall of zeros, so this turns "now what?" into three concrete clicks across the
// two catalogs (infrastructure + AI). Steps mark themselves done as the install
// grows; the card disappears once a real workload exists.
export function GetStarted({
  hasInfraProvider,
  hasAIProvider,
  hasAnyRequest,
}: {
  hasInfraProvider: boolean;
  hasAIProvider: boolean;
  hasAnyRequest: boolean;
}) {
  const steps: Step[] = [
    {
      icon: PlugZap,
      title: "Connect a provider",
      body: "Register an infrastructure backend (Proxmox, vSphere, AWS, Azure, GCP). Credentials stay in your OpenBao.",
      href: "/providers",
      label: "Providers",
      done: hasInfraProvider,
    },
    {
      icon: Sparkles,
      title: "Connect an AI provider",
      body: "Add OpenAI or Anthropic by secret reference - or explore the flow with the seeded MockAI first.",
      href: "/ai/providers",
      label: "AI providers",
      done: hasAIProvider,
    },
    {
      icon: LayoutGrid,
      title: "Request from a catalog",
      body: "Browse the infrastructure catalog or request governed AI access - every grant is approved and audited.",
      href: "/catalog",
      label: "Open catalog",
      done: hasAnyRequest,
    },
  ];

  return (
    <Card className="border-primary/20 bg-gradient-to-br from-primary/[0.06] to-transparent">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <ShieldCheck className="size-5 text-primary" />
          Get started with OPORD
        </CardTitle>
        <CardDescription>
          One governed workflow for two catalogs - infrastructure and AI access. Three steps to your
          first request.
        </CardDescription>
      </CardHeader>
      <CardContent className="grid gap-3 md:grid-cols-3">
        {steps.map((s, i) => (
          <div
            key={s.title}
            className="relative flex flex-col gap-2 rounded-xl border border-border bg-card p-4"
          >
            <div className="flex items-center justify-between">
              <span
                className={cn(
                  "flex size-9 items-center justify-center rounded-lg",
                  s.done ? "bg-success/10 text-success" : "bg-primary/10 text-primary",
                )}
              >
                {s.done ? <CheckCircle2 className="size-5" /> : <s.icon className="size-5" />}
              </span>
              <span className="text-xs font-semibold tabular-nums text-muted-foreground">
                {i + 1}/3
              </span>
            </div>
            <div className="text-sm font-semibold">{s.title}</div>
            <p className="flex-1 text-sm text-muted-foreground">{s.body}</p>
            <Link
              href={s.href}
              className={cn(button({ variant: s.done ? "outline" : "primary", size: "sm" }), "mt-1 self-start")}
            >
              {s.label}
              <ArrowRight className="size-3.5" />
            </Link>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}
