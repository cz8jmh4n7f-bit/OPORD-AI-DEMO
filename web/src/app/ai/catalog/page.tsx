import { AIRequestButton } from "@/components/ai-request-button";
import { EmptyState } from "@/components/empty-state";
import { PageHeader } from "@/components/page-header";
import { fetchAIServices } from "@/lib/api";

export const metadata = { title: "AI Services" };

export default async function AICatalogPage() {
  const services = await fetchAIServices();

  return (
    <div className="space-y-5">
      <PageHeader
        title="AI Services"
        description="Request governed AI access through OPORD's existing approval workflow."
      />

      {services.length === 0 ? (
        <EmptyState
          title="No AI services yet"
          description="Run the AI migration or add a mock AI provider to populate the catalog."
          action={{ href: "/ai/providers", label: "AI providers" }}
        />
      ) : (
        // Provider-first catalog as a list (not equal cards): each provider is a
        // bordered group; each service is a row with a ghost request link.
        <div className="space-y-6">
          {groupByProvider(services).map((group) => (
            <section key={group.providerName} className="space-y-2">
              <div className="flex items-center gap-2 px-1">
                <h2 className="text-[13px] font-medium text-foreground">{group.providerName}</h2>
                <span className="rounded border border-border px-1.5 py-0.5 font-mono text-[12px] text-faint">
                  {group.providerType}
                </span>
                <span className="text-xs text-faint">
                  {group.items.length} service{group.items.length === 1 ? "" : "s"}
                </span>
              </div>

              <div className="divide-y divide-border overflow-hidden rounded-lg border border-border bg-surface-2">
                {group.items.map((service) => (
                  <div
                    key={service.id}
                    className="flex items-start justify-between gap-4 px-4 py-3.5 transition-colors hover:bg-surface-3"
                  >
                    <div className="min-w-0 flex-1">
                      <h3 className="text-[14px] font-medium text-foreground">{service.name}</h3>
                      <p className="mt-0.5 text-[13px] leading-5 text-muted-foreground">{service.description}</p>
                      <div className="mt-2 flex flex-wrap items-center gap-1.5">
                        <Tag>{service.category}</Tag>
                        <Tag>{service.requiresApproval ? "approval required" : "auto-approved"}</Tag>
                        <Tag>{service.defaultExpirationDays}d default</Tag>
                        <span className="ml-1 font-mono text-[11px] text-faint">{service.slug}</span>
                      </div>
                    </div>
                    <div className="shrink-0 pt-0.5">
                      <AIRequestButton service={service} />
                    </div>
                  </div>
                ))}
              </div>
            </section>
          ))}
        </div>
      )}
    </div>
  );
}

// Tag — a tiny monospace, bordered label (not a colored pill).
function Tag({ children }: { children: React.ReactNode }) {
  return (
    <span className="rounded border border-border px-1.5 py-0.5 font-mono text-[12px] text-muted-foreground">
      {children}
    </span>
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
