import Link from "next/link";
import { KeyRound, Plus } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { EmptyState } from "@/components/empty-state";
import { ClusterStatusBadge } from "@/components/status-badge";
import { DestroyButton } from "@/components/destroy-button";
import { DeployTarget } from "@/components/deploy-target";
import { fetchSecrets } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

export const metadata = { title: "Secrets" };

export default async function SecretsPage() {
  const secrets = await fetchSecrets();

  return (
    <div className="space-y-6">
      <PageHeader
        title="Secrets"
        description="Managed secret containers (AWS Secrets Manager / Azure Key Vault / GCP Secret Manager). OPORD provisions the container only - values are set out-of-band, so OPORD never holds plaintext."
      >
        <Link href="/secrets/new" className={button({ size: "sm" })}>
          <Plus className="size-4" />
          New secret
        </Link>
      </PageHeader>

      {secrets.length === 0 ? (
        <Card>
          <EmptyState
            icon={KeyRound}
            title="No secrets yet"
            description="Provision a managed secret from the catalog or CLI (opord secret create)."
            action={{ href: "/secrets/new", label: "New secret" }}
          />
        </Card>
      ) : (
        <Card className="overflow-hidden p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
                  <th scope="col" className="px-5 py-3 font-medium">Name</th>
                  <th scope="col" className="px-5 py-3 font-medium">Provider</th>
                  <th scope="col" className="px-5 py-3 font-medium">Secret</th>
                  <th scope="col" className="px-5 py-3 font-medium">ARN / ID</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Created</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {secrets.map((secret) => (
                  <tr key={secret.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <span className="font-medium">{secret.name}</span>
                      <div className="text-xs text-muted-foreground">{secret.environment}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {secret.provider}
                      <DeployTarget account={secret.targetAccount} />
                    </td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">{secret.secretName}</td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">
                      {secret.secretArn ? secret.secretArn.split("/").pop() || secret.secretArn : "-"}
                    </td>
                    <td className="px-5 py-3">
                      <ClusterStatusBadge status={secret.status} error={secret.lastError} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{timeAgo(secret.createdAt)}</td>
                    <td className="sticky right-0 bg-card px-5 py-3 text-right">
                      <DestroyButton resource="secrets" name={secret.name} environment={secret.environment} status={secret.status} />
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
