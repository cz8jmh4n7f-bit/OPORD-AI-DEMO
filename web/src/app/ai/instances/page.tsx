import { Bot, CheckCircle2, ShieldAlert } from "lucide-react";
import { AIInstanceActions } from "@/components/ai-instance-actions";
import { EmptyState } from "@/components/empty-state";
import { PageHeader } from "@/components/page-header";
import { StatCard } from "@/components/stat-card";
import { Badge, type BadgeVariant } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { fetchAIInstances } from "@/lib/api";
import { formatDate } from "@/lib/utils";

export const metadata = { title: "AI Access" };

const statusVariant: Record<string, BadgeVariant> = {
  active: "success",
  provisioning: "info",
  suspended: "warning",
  revoking: "warning",
  revoked: "default",
  expired: "default",
  failed: "danger",
};

export default async function AIInstancesPage() {
  const instances = await fetchAIInstances();
  const active = instances.filter((i) => i.status === "active").length;
  const revoked = instances.filter((i) => i.status === "revoked").length;

  return (
    <div className="space-y-6">
      <PageHeader title="AI Access" description="Provisioned AI service instances, ownership, expiry, and revocation." />

      <div className="grid gap-3 sm:grid-cols-3">
        <StatCard icon={Bot} label="Instances" value={instances.length} hint="All AI access records" />
        <StatCard icon={CheckCircle2} label="Active" value={active} hint="Currently granted" accent="bg-success/10 text-success" />
        <StatCard icon={ShieldAlert} label="Revoked" value={revoked} hint="Access removed" />
      </div>

      {instances.length === 0 ? (
        <Card>
          <EmptyState
            icon={Bot}
            title="No AI access instances"
            description="Approved AI requests will create mock service instances here."
            action={{ href: "/ai/catalog", label: "Request AI access" }}
          />
        </Card>
      ) : (
        <Card className="overflow-hidden p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
                  <th scope="col" className="px-5 py-3 font-medium">Service</th>
                  <th scope="col" className="px-5 py-3 font-medium">Owner</th>
                  <th scope="col" className="px-5 py-3 font-medium">Workspace</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Expires</th>
                  <th scope="col" className="px-5 py-3 font-medium">Provider Access</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {instances.map((i) => (
                  <tr key={i.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <div className="font-medium">{i.serviceName}</div>
                      <div className="text-xs text-muted-foreground">{i.providerName}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{i.owner || "-"}</td>
                    <td className="px-5 py-3 text-muted-foreground">{i.workspace || "-"}</td>
                    <td className="px-5 py-3">
                      <Badge variant={statusVariant[i.status] ?? "default"}>{i.status}</Badge>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{i.expiresAt ? formatDate(i.expiresAt) : "-"}</td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">{i.providerAccessId || "-"}</td>
                    <td className="sticky right-0 bg-card px-5 py-3 text-right">
                      <AIInstanceActions id={i.id} status={i.status} />
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
