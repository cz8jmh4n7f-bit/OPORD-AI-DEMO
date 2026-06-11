"use client";

import { Check, Construction, Sparkles } from "lucide-react";
import { useRouter } from "next/navigation";
import { setAIMode } from "@/lib/ai-mode";

// ComingSoon is shown in the opord-ai (AI-first) build when the topbar "AI" sign
// is off: the cloud / on-prem infrastructure marketplace is still in development,
// so the console surfaces this instead of the infra pages. The CTA flips the sign
// on and drops you into the AI workspace.
export function ComingSoon() {
  const router = useRouter();

  function enterAI() {
    setAIMode(true);
    router.push("/ai/overview");
  }

  return (
    <div className="grid min-h-[70vh] place-items-center px-4">
      <div className="mx-auto max-w-xl text-center">
        <div className="mx-auto mb-6 grid size-16 place-items-center rounded-2xl bg-primary/10 text-primary">
          <Construction className="size-8" />
        </div>
        <h1 className="text-2xl font-bold text-foreground sm:text-3xl">
          Cloud &amp; on-prem catalog - in development
        </h1>
        <p className="mt-4 text-base leading-7 text-muted-foreground">
          OPORD&apos;s infrastructure marketplace covers VMs, Kubernetes clusters,
          databases, and networks across AWS, Azure, GCP, vSphere, and Proxmox, and
          is on the way. This build focuses on{" "}
          <span className="font-medium text-foreground">AI Service Governance</span>.
        </p>
        <ul className="mx-auto mt-6 max-w-md space-y-2 text-left text-sm text-muted-foreground">
          {[
            "Self-service AI requests with approval and expiry",
            "Policies, seat quotas, and budgets enforced on every grant",
            "Full audit trail, usage metering, and live key validation",
          ].map((t) => (
            <li key={t} className="flex items-start gap-2">
              <Check className="mt-0.5 size-4 shrink-0 text-primary" />
              <span>{t}</span>
            </li>
          ))}
        </ul>
        <button
          type="button"
          onClick={enterAI}
          className="mt-8 inline-flex items-center gap-2 rounded-lg bg-primary px-5 py-2.5 text-sm font-semibold text-primary-foreground shadow-md transition-colors hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-background"
        >
          <Sparkles className="size-4" />
          Enter the AI workspace
        </button>
        <p className="mt-4 text-xs text-muted-foreground">
          or click the <span className="font-semibold text-foreground">AI</span> sign in the top bar
        </p>
      </div>
    </div>
  );
}
