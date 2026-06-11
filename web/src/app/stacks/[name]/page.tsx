import Link from "next/link";
import { notFound } from "next/navigation";
import { ChevronLeft } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { ClusterStatusBadge } from "@/components/status-badge";
import { DestroyButton } from "@/components/destroy-button";
import { fetchStack } from "@/lib/api";
import { cn, formatDate } from "@/lib/utils";

export const metadata = { title: "Stacks" };

function Info({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <dt className="text-xs uppercase tracking-wide text-muted-foreground">{label}</dt>
      <dd className={cn("mt-1 text-sm font-medium", mono && "font-mono break-all")}>{value}</dd>
    </div>
  );
}

function KVTable({ title, data }: { title: string; data: Record<string, unknown> }) {
  const entries = Object.entries(data);
  return (
    <Card className="overflow-hidden p-0">
      <CardHeader className="p-5 pb-3">
        <CardTitle>
          {title} <span className="text-muted-foreground">({entries.length})</span>
        </CardTitle>
      </CardHeader>
      {entries.length === 0 ? (
        <p className="px-5 pb-5 text-sm text-muted-foreground">None.</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <tbody>
              {entries.map(([k, v]) => (
                <tr key={k} className="border-t border-border">
                  <td className="px-5 py-2.5 font-medium align-top w-1/3">{k}</td>
                  <td className="px-5 py-2.5 font-mono text-xs text-muted-foreground break-all">
                    {typeof v === "string" ? v : JSON.stringify(v)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Card>
  );
}

export default async function StackDetailPage({ params }: { params: Promise<{ name: string }> }) {
  const { name } = await params;
  const stack = await fetchStack(decodeURIComponent(name));
  if (!stack) notFound();

  return (
    <div className="space-y-6">
      <Link href="/stacks" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ChevronLeft className="size-4" />
        Stacks
      </Link>

      <PageHeader title={stack.name} description={`${stack.environment} · ${stack.provider}`}>
        <ClusterStatusBadge status={stack.status} error={stack.lastError} />
        <DestroyButton resource="stacks" name={stack.name} environment={stack.environment} status={stack.status} />
      </PageHeader>

      <Card>
        <dl className="grid grid-cols-1 gap-5 p-5 sm:grid-cols-3">
          <Info label="Provider" value={stack.provider || "-"} />
          <Info
            label="Deploy target"
            value={stack.targetAccount || "Provider default"}
            mono={Boolean(stack.targetAccount)}
          />
          <Info label="Module" value={stack.moduleDir} mono />
          <Info label="Created" value={formatDate(stack.createdAt)} />
        </dl>
      </Card>

      <KVTable title="Variables" data={stack.variables ?? {}} />
      <KVTable title="Outputs" data={stack.outputs ?? {}} />
    </div>
  );
}
