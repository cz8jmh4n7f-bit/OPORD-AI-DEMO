import Link from "next/link";
import { Cpu } from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { PageHeader } from "@/components/page-header";
import { Badge, type BadgeVariant } from "@/components/ui/badge";
import { button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { fetchAIModels } from "@/lib/api";
import { formatDate } from "@/lib/utils";

export const metadata = { title: "AI Models" };

// Color the modality badge by value so the column carries meaning at a glance
// (previously every row was rendered with the same "info" variant).
const modalityVariant: Record<string, BadgeVariant> = {
  text: "info",
  chat: "info",
  code: "info",
  image: "success",
  vision: "success",
  multimodal: "success",
  embedding: "warning",
  embeddings: "warning",
  audio: "default",
};

export default async function AIModelsPage() {
  const models = await fetchAIModels();

  return (
    <div className="space-y-6">
      <PageHeader title="AI Models" description="Approved and discovered AI model catalog across providers.">
        <Link href="/ai/providers" className={button({ variant: "outline", size: "sm" })}>
          Sync in AI Providers
        </Link>
      </PageHeader>

      {models.length === 0 ? (
        <Card>
          <EmptyState icon={Cpu} title="No AI models" description="Use AI Providers to sync model catalog entries from MockAI, OpenAI, or Anthropic." />
        </Card>
      ) : (
        <Card className="overflow-hidden p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
                  <th scope="col" className="px-5 py-3 font-medium">Model</th>
                  <th scope="col" className="px-5 py-3 font-medium">Provider</th>
                  <th scope="col" className="px-5 py-3 font-medium">Modality</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Updated</th>
                </tr>
              </thead>
              <tbody>
                {models.map((m) => (
                  <tr key={m.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <div className="font-medium">{m.displayName || m.model}</div>
                      <div className="font-mono text-xs text-muted-foreground">{m.model}</div>
                    </td>
                    <td className="px-5 py-3">
                      <div className="text-muted-foreground">{m.providerName}</div>
                      <div className="font-mono text-xs text-muted-foreground">{m.providerType}</div>
                    </td>
                    <td className="px-5 py-3"><Badge variant={modalityVariant[m.modality?.toLowerCase()] ?? "default"}>{m.modality}</Badge></td>
                    <td className="px-5 py-3"><Badge variant={m.status === "active" ? "success" : "default"}>{m.status}</Badge></td>
                    <td className="px-5 py-3 text-muted-foreground">{formatDate(m.updatedAt)}</td>
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
