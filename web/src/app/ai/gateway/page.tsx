import { KeyRound, ShieldCheck, WandSparkles } from "lucide-react";
import { GatewaySmokeButton } from "@/components/ai-governance-actions";
import { PageHeader } from "@/components/page-header";
import { StatCard } from "@/components/stat-card";
import { Card } from "@/components/ui/card";
import { fetchAIProviders } from "@/lib/api";

export const metadata = { title: "AI Gateway" };

export default async function AIGatewayPage() {
  const providers = await fetchAIProviders();
  const openai = providers.find((p) => p.type === "openai");

  return (
    <div className="space-y-6">
      <PageHeader title="AI Gateway" description="A lightweight governed OpenAI Responses proxy using provider secrets stored outside Postgres.">
        <GatewaySmokeButton provider={openai?.name} />
      </PageHeader>

      <div className="grid gap-3 sm:grid-cols-3">
        <StatCard icon={ShieldCheck} label="Prompt logging" value="Off" hint="Audit stores metadata only" />
        <StatCard icon={KeyRound} label="Key handling" value="OpenBao" hint="Provider secret_ref is used" accent="bg-success/10 text-success" />
        <StatCard icon={WandSparkles} label="Endpoint" value="/responses" hint="OpenAI Responses API proxy" accent="bg-info/10 text-info" />
      </div>

      <Card className="p-5">
        <h2 className="text-sm font-semibold text-foreground">Gateway contract</h2>
        <p className="mt-2 text-sm leading-6 text-muted-foreground">
          Applications can call OPORD instead of receiving raw OpenAI keys. OPORD forwards the JSON body to OpenAI, records audit metadata,
          and writes token usage when the provider response includes usage fields. Budgets and token / cost / request quotas are
          enforced on this gateway path - a call is refused once a matching limit is exhausted.
        </p>
        <pre className="mt-4 overflow-auto rounded-lg bg-muted p-3 text-xs text-muted-foreground">
{`POST /api/v1/ai/gateway/openai/responses?provider=openai-main
Authorization: Bearer <opord-api-key>
Content-Type: application/json

{
  "model": "gpt-5-mini",
  "input": "Say hello from OPORD"
}`}
        </pre>
      </Card>
    </div>
  );
}
