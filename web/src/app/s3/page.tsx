import Link from "next/link";
import { Archive, Plus } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { EmptyState } from "@/components/empty-state";
import { ClusterStatusBadge } from "@/components/status-badge";
import { DestroyButton } from "@/components/destroy-button";
import { DeployTarget } from "@/components/deploy-target";
import { fetchS3Buckets } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

export const metadata = { title: "Object storage" };

export default async function S3Page() {
  const buckets = await fetchS3Buckets();

  return (
    <div className="space-y-6">
      <PageHeader
        title="Object storage"
        description="Managed private object storage (S3 / GCS / Azure Blob) with versioning and public-access block."
      >
        <Link href="/s3/new" className={button({ size: "sm" })}>
          <Plus className="size-4" />
          New bucket
        </Link>
      </PageHeader>

      {buckets.length === 0 ? (
        <Card>
          <EmptyState
            icon={Archive}
            title="No S3 buckets yet"
            description="Provision a private, versioned bucket from the catalog or CLI (opord s3 create)."
            action={{ href: "/s3/new", label: "New bucket" }}
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
                  <th scope="col" className="px-5 py-3 font-medium">Bucket</th>
                  <th scope="col" className="px-5 py-3 font-medium">Safety</th>
                  <th scope="col" className="px-5 py-3 font-medium">ARN</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Created</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {buckets.map((bucket) => (
                  <tr key={bucket.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <span className="font-medium">{bucket.name}</span>
                      <div className="text-xs text-muted-foreground">{bucket.environment}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {bucket.provider}
                      <DeployTarget account={bucket.targetAccount} />
                    </td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">{bucket.bucketName}</td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {bucket.blockPublicAccess ? "private" : "public allowed"}
                      {bucket.versioning ? " / versioned" : ""}
                    </td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">
                      {bucket.bucketArn ? bucket.bucketArn.split(":bucket/")[1] || bucket.bucketArn : "-"}
                    </td>
                    <td className="px-5 py-3">
                      <ClusterStatusBadge status={bucket.status} error={bucket.lastError} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{timeAgo(bucket.createdAt)}</td>
                    <td className="sticky right-0 bg-card px-5 py-3 text-right">
                      <DestroyButton resource="s3" name={bucket.name} environment={bucket.environment} status={bucket.status} />
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
