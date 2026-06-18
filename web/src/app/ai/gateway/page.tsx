import { GatewaySmokeButton } from "@/components/ai-governance-actions";
import { CopyButton } from "@/components/copy-button";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";

export const metadata = { title: "AI Gateway" };

const SNIPPET = `POST /api/v1/ai/gateway/openai/responses?provider=openai-main
Authorization: Bearer <opord-api-key>
Content-Type: application/json

{
  "model": "gpt-5-mini",
  "input": "Say hello from OPORD"
}`;

const facts = [
  { label: "Prompt logging", value: "Off", hint: "Audit stores metadata only" },
  { label: "Key handling", value: "OpenBao", hint: "Provider secret_ref is used" },
  { label: "Endpoint", value: "/responses", hint: "OpenAI Responses API proxy" },
];

export default function AIGatewayPage() {
  return (
    <div className="space-y-5">
      <PageHeader
        title="AI Gateway"
        description="A lightweight governed OpenAI Responses proxy using provider secrets stored outside Postgres."
      >
        <GatewaySmokeButton />
      </PageHeader>

      {/* One horizontal row, segments divided — not three separate cards. */}
      <div className="grid divide-y divide-border rounded-lg border border-border bg-surface-2 sm:grid-cols-3 sm:divide-x sm:divide-y-0">
        {facts.map((f) => (
          <div key={f.label} className="p-4">
            <div className="text-[11px] font-medium uppercase tracking-[0.06em] text-muted-foreground">{f.label}</div>
            <div className="mt-2 text-[15px] font-medium text-foreground">{f.value}</div>
            <div className="mt-0.5 text-xs text-faint">{f.hint}</div>
          </div>
        ))}
      </div>

      <Card className="p-5">
        <h2 className="text-[13px] font-medium text-foreground">Gateway contract</h2>
        <p className="mt-2 max-w-prose text-[13px] leading-6 text-muted-foreground">
          Applications call OPORD instead of receiving raw OpenAI keys. OPORD forwards the JSON body to OpenAI, records
          audit metadata, and writes token usage when the provider response includes usage fields. Budgets and quotas
          are visible now; hard enforcement can be tightened around this gateway path next.
        </p>

        {/* The signature dark code block — kept dark in either theme. */}
        <div className="relative mt-4 overflow-hidden rounded-lg border border-white/10 bg-[#0a0a0a]">
          <CopyButton text={SNIPPET} className="absolute right-2 top-2" />
          <pre className="overflow-auto p-4 pr-12 font-mono text-xs leading-6 text-[#d4d4d4]">{SNIPPET}</pre>
        </div>
      </Card>
    </div>
  );
}
