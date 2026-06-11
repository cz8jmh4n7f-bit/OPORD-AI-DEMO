import { Bot, Server } from "lucide-react";
import { AddAIProviderButton, AIProviderActions } from "@/components/ai-provider-actions";
import { EmptyState } from "@/components/empty-state";
import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { fetchAIProviders } from "@/lib/api";
import { formatDate } from "@/lib/utils";

export const metadata = { title: "AI Providers" };

export default async function AIProvidersPage() {
  const providers = await fetchAIProviders();

  return (
    <div className="space-y-6">
      <PageHeader title="AI Providers" description="Configured AI governance providers for MockAI, OpenAI, ChatGPT, Anthropic, and Claude Code access.">
        <AddAIProviderButton />
      </PageHeader>

      {providers.length === 0 ? (
        <Card>
          <EmptyState
            icon={Bot}
            title="No AI providers"
            description="Add MockAI for local testing, or register OpenAI/Anthropic with a secret reference for credential validation."
          />
        </Card>
      ) : (
        <Card className="overflow-hidden p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
                  <th scope="col" className="px-5 py-3 font-medium">Provider</th>
                  <th scope="col" className="px-5 py-3 font-medium">Type</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Added</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {providers.map((p) => (
                  <tr key={p.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <div className="flex items-center gap-2 font-medium">
                        <Server className="size-4 text-primary" />
                        {p.name}
                      </div>
                    </td>
                    <td className="px-5 py-3"><Badge variant="info">{p.type}</Badge></td>
                    <td className="px-5 py-3"><Badge variant={p.status === "active" ? "success" : "default"}>{p.status}</Badge></td>
                    <td className="px-5 py-3 text-muted-foreground">{formatDate(p.createdAt)}</td>
                    <td className="sticky right-0 bg-card px-5 py-3 text-right">
                      <AIProviderActions provider={p} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}
    </div>
  );
}
