import Link from "next/link";
import { Zap, Plus } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { EmptyState } from "@/components/empty-state";
import { ClusterStatusBadge } from "@/components/status-badge";
import { DestroyButton } from "@/components/destroy-button";
import { DeployTarget } from "@/components/deploy-target";
import { fetchFunctions } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

export const metadata = { title: "Functions" };

export default async function FunctionsPage() {
  const functions = await fetchFunctions();

  return (
    <div className="space-y-6">
      <PageHeader
        title="Functions"
        description="Serverless functions (AWS Lambda / GCP Cloud Functions / Azure Functions) with an auto-created execution role."
      >
        <Link href="/functions/new" className={button({ size: "sm" })}>
          <Plus className="size-4" />
          New function
        </Link>
      </PageHeader>

      {functions.length === 0 ? (
        <Card>
          <EmptyState
            icon={Zap}
            title="No functions yet"
            description="Provision a Lambda - OPORD auto-creates the execution role and (with no code supplied) ships a built-in handler so it's immediately invokable."
            action={{ href: "/functions/new", label: "New function" }}
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
                  <th scope="col" className="px-5 py-3 font-medium">Runtime</th>
                  <th scope="col" className="px-5 py-3 font-medium">Memory</th>
                  <th scope="col" className="px-5 py-3 font-medium">ARN</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Created</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {functions.map((fn) => (
                  <tr key={fn.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                    <td className="px-5 py-3">
                      <span className="font-medium">{fn.name}</span>
                      <div className="text-xs text-muted-foreground">{fn.environment}</div>
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {fn.provider}
                      <DeployTarget account={fn.targetAccount} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{fn.runtime}</td>
                    <td className="px-5 py-3 text-muted-foreground">{fn.memoryMb ? `${fn.memoryMb} MB` : "-"}</td>
                    <td className="px-5 py-3 font-mono text-xs text-muted-foreground">
                      {fn.arn ? fn.arn.split(":function:")[1] || fn.arn : "-"}
                    </td>
                    <td className="px-5 py-3">
                      <ClusterStatusBadge status={fn.status} error={fn.lastError} />
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{timeAgo(fn.createdAt)}</td>
                    <td className="sticky right-0 bg-card px-5 py-3 text-right">
                      <DestroyButton resource="functions" name={fn.name} environment={fn.environment} status={fn.status} />
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
