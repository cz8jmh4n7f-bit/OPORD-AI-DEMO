import Link from "next/link";
import { BadgeCheck, Plus } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { EmptyState } from "@/components/empty-state";
import { ClusterStatusBadge } from "@/components/status-badge";
import { DestroyButton } from "@/components/destroy-button";
import { DeployTarget } from "@/components/deploy-target";
import { fetchCerts } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

export const metadata = { title: "Certificates" };

export default async function CertsPage() {
  const certs = await fetchCerts();

  return (
    <div className="space-y-6">
      <PageHeader
        title="Certificates"
        description="Managed TLS certificates (ACM) with DNS validation for the expose layer."
      >
        <Link href="/certs/new" className={button({ size: "sm" })}>
          <Plus className="size-4" />
          New certificate
        </Link>
      </PageHeader>

      {certs.length === 0 ? (
        <Card>
          <EmptyState
            icon={BadgeCheck}
            title="No certificates yet"
            description="Request an ACM certificate from the catalog or CLI (opord cert create)."
            action={{ href: "/certs/new", label: "New certificate" }}
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
                  <th scope="col" className="px-5 py-3 font-medium">Domain</th>
                  <th scope="col" className="px-5 py-3 font-medium">Cert status</th>
                  <th scope="col" className="px-5 py-3 font-medium">ARN</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Created</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {certs.map((cert) => (
                  <tr key={cert.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <span className="font-medium">{cert.name}</span>
                      <div className="text-xs text-muted-foreground">{cert.environment}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {cert.provider}
                      <DeployTarget account={cert.targetAccount} />
                    </td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">{cert.domain}</td>
                    <td className="px-5 py-3 text-muted-foreground">{cert.certStatus ?? "-"}</td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">
                      {cert.arn ? cert.arn.split("/")[1] || cert.arn : "-"}
                    </td>
                    <td className="px-5 py-3">
                      <ClusterStatusBadge status={cert.status} error={cert.lastError} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{timeAgo(cert.createdAt)}</td>
                    <td className="sticky right-0 bg-card px-5 py-3 text-right">
                      <DestroyButton resource="certs" name={cert.name} environment={cert.environment} status={cert.status} />
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
