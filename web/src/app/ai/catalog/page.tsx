import { Bot, Sparkles } from "lucide-react";
import { AIRequestButton } from "@/components/ai-request-button";
import { EmptyState } from "@/components/empty-state";
import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { fetchAIServices } from "@/lib/api";

export const metadata = { title: "AI Services" };

export default async function AICatalogPage() {
  const services = await fetchAIServices();

  return (
    <div className="space-y-6">
      <PageHeader title="AI Services" description="Request governed AI access through OPORD's existing approval workflow." />

      {services.length === 0 ? (
        <Card>
          <EmptyState
            icon={Bot}
            title="No AI services yet"
            description="Run the AI migration or add a mock AI provider to populate the MVP catalog."
            action={{ href: "/ai/providers", label: "AI providers" }}
          />
        </Card>
      ) : (
        // Provider-first: group the catalog by provider so it's clear which
        // services belong to Anthropic / LiteLLM / a mock, etc. (the same
        // provider-first shape as the infra catalog). Stable, first-appearance order.
        <div className="space-y-8">
          {groupByProvider(services).map((group) => (
            <section key={group.providerName} className="space-y-3">
              <div className="flex items-center gap-2 border-b border-border pb-2">
                <h2 className="text-sm font-semibold text-foreground">{group.providerName}</h2>
                <Badge variant="info">{group.providerType}</Badge>
                <span className="text-xs text-muted-foreground">
                  {group.items.length} service{group.items.length === 1 ? "" : "s"}
                </span>
              </div>
              <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                {group.items.map((service) => (
                  <Card key={service.id} className="flex h-full flex-col p-4">
                    <div className="flex items-start gap-3">
                      <div className="grid size-10 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">
                        <Sparkles className="size-5" />
                      </div>
                      <div className="min-w-0 flex-1">
                        <h3 className="text-sm font-semibold text-foreground">{service.name}</h3>
                      </div>
                    </div>
                    <p className="mt-3 min-h-12 text-sm leading-6 text-muted-foreground">{service.description}</p>
                    <div className="mt-3 flex flex-wrap gap-1.5">
                      <Badge variant="info">{service.category}</Badge>
                      <Badge variant={service.requiresApproval ? "warning" : "success"}>
                        {service.requiresApproval ? "Approval required" : "Auto-approved"}
                      </Badge>
                      <Badge variant="default">{service.defaultExpirationDays}d default</Badge>
                    </div>
                    <div className="mt-4 flex items-center justify-between border-t border-border pt-3">
                      <span className="font-mono text-xs text-muted-foreground">{service.slug}</span>
                      <AIRequestButton service={service} />
                    </div>
                  </Card>
                ))}
              </div>
            </section>
          ))}
        </div>
      )}
    </div>
  );
}

// groupByProvider buckets services under their provider, preserving the order each
// provider first appears (so the layout stays stable as the catalog grows).
function groupByProvider(services: Awaited<ReturnType<typeof fetchAIServices>>) {
  const groups: { providerName: string; providerType: string; items: typeof services }[] = [];
  for (const s of services) {
    let g = groups.find((x) => x.providerName === s.providerName);
    if (!g) {
      g = { providerName: s.providerName, providerType: s.providerType, items: [] };
      groups.push(g);
    }
    g.items.push(s);
  }
  return groups;
}
