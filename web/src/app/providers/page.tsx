import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { ProviderActions } from "@/components/provider-actions";
import { ProviderHealthBadge } from "@/components/provider-health-badge";
import { AddProviderButton } from "@/components/add-provider-button";
import { fetchProviders } from "@/lib/api";
import { formatDate } from "@/lib/utils";

export const metadata = { title: "Providers" };

export default async function ProvidersPage() {
  const providers = await fetchProviders();

  return (
    <div className="space-y-6">
      <PageHeader
        title="Providers"
        description="Configured infrastructure backends. Credentials are resolved from OpenBao (Vault-compatible) at run time."
      >
        <AddProviderButton />
      </PageHeader>

      <Card className="overflow-hidden p-0">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
                <th scope="col" className="px-5 py-3 font-medium">Name</th>
                <th scope="col" className="px-5 py-3 font-medium">Type</th>
                <th scope="col" className="px-5 py-3 font-medium">Server</th>
                <th scope="col" className="px-5 py-3 font-medium">Datacenter</th>
                <th scope="col" className="px-5 py-3 font-medium">Clusters</th>
                <th scope="col" className="px-5 py-3 font-medium">Health</th>
                <th scope="col" className="px-5 py-3 font-medium">Added</th>
                <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {providers.length === 0 && (
                <tr>
                  <td colSpan={8} className="px-5 py-6 text-center text-muted-foreground">
                    No providers registered.
                  </td>
                </tr>
              )}
              {providers.map((p) => (
                <tr key={p.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                  <td className="px-5 py-3 font-medium">{p.name}</td>
                  <td className="px-5 py-3">
                    <Badge variant={p.type === "vsphere" ? "info" : "primary"}>{p.type}</Badge>
                  </td>
                  <td className="px-5 py-3 font-mono text-xs text-muted-foreground">{p.server || "-"}</td>
                  <td className="px-5 py-3 text-muted-foreground">{p.datacenter || "-"}</td>
                  <td className="px-5 py-3 text-muted-foreground">{p.clusters}</td>
                  <td className="px-5 py-3">
                    <ProviderHealthBadge health={p.health} />
                  </td>
                  <td className="px-5 py-3 text-muted-foreground">{formatDate(p.createdAt)}</td>
                  <td className="sticky right-0 bg-card px-5 py-3">
                    <ProviderActions provider={p} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>
    </div>
  );
}
