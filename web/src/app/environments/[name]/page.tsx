import Link from "next/link";
import { notFound } from "next/navigation";
import { ChevronLeft } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge, type BadgeVariant } from "@/components/ui/badge";
import { ClusterStatusBadge } from "@/components/status-badge";
import { DestroyButton } from "@/components/destroy-button";
import { fetchEnvironment } from "@/lib/api";
import { cn, formatDate } from "@/lib/utils";

export const metadata = { title: "Environments" };

// Component status tolerates "missing" (child not created yet), so it can't use
// the ClusterStatus-typed badge directly.
const compVariant: Record<string, BadgeVariant> = {
  ready: "success",
  provisioning: "info",
  bootstrapping: "warning",
  pending: "default",
  degraded: "warning",
  destroying: "warning",
  destroyed: "default",
  failed: "danger",
  missing: "default",
};

function Info({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <dt className="text-xs uppercase tracking-wide text-muted-foreground">{label}</dt>
      <dd className={cn("mt-1 text-sm font-medium", mono && "font-mono")}>{value}</dd>
    </div>
  );
}

export default async function EnvironmentDetailPage({ params }: { params: Promise<{ name: string }> }) {
  const { name } = await params;
  const env = await fetchEnvironment(decodeURIComponent(name));
  if (!env) notFound();

  return (
    <div className="space-y-6">
      <Link
        href="/environments"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ChevronLeft className="size-4" />
        Environments
      </Link>

      <PageHeader title={env.name} description={`${env.environment} · ${env.blueprint} · ${env.provider}`}>
        <ClusterStatusBadge status={env.status} />
        <DestroyButton resource="environments" name={env.name} environment={env.environment} status={env.status} />
      </PageHeader>

      <Card>
        <dl className="grid grid-cols-2 gap-5 p-5 sm:grid-cols-4">
          <Info label="Blueprint" value={env.blueprint} />
          <Info label="Provider" value={env.provider || "-"} />
          <Info label="Components" value={String(env.components.length)} />
          <Info label="Created" value={formatDate(env.createdAt)} />
        </dl>
      </Card>

      <Card className="overflow-hidden p-0">
        <CardHeader className="p-5 pb-3">
          <CardTitle>
            Components <span className="text-muted-foreground">({env.components.length})</span>
          </CardTitle>
        </CardHeader>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-y border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
                <th scope="col" className="px-5 py-2.5 font-medium">Component</th>
                <th scope="col" className="px-5 py-2.5 font-medium">Kind</th>
                <th scope="col" className="px-5 py-2.5 font-medium">Resource</th>
                <th scope="col" className="px-5 py-2.5 font-medium">Status</th>
              </tr>
            </thead>
            <tbody>
              {env.components.map((c) => (
                <tr key={c.name} className="border-b border-border last:border-0">
                  <td className="px-5 py-2.5 font-medium">{c.name}</td>
                  <td className="px-5 py-2.5">
                    <Badge variant={c.kind === "k8s-cluster" ? "primary" : "default"}>{c.kind}</Badge>
                  </td>
                  <td className="px-5 py-2.5 font-mono text-xs text-muted-foreground">{c.resource}</td>
                  <td className="px-5 py-2.5">
                    <Badge variant={compVariant[c.status] ?? "default"}>{c.status}</Badge>
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
