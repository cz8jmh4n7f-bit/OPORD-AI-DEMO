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
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {services.map((service) => (
            <Card key={service.id} className="flex h-full flex-col p-4">
              <div className="flex items-start gap-3">
                <div className="grid size-10 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">
                  <Sparkles className="size-5" />
                </div>
                <div className="min-w-0 flex-1">
                  <h2 className="text-sm font-semibold text-foreground">{service.name}</h2>
                  <p className="mt-1 text-xs text-muted-foreground">{service.providerName} · {service.providerType}</p>
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
      )}
    </div>
  );
}
