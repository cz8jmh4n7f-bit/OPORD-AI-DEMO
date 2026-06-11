import Link from "next/link";
import { notFound } from "next/navigation";
import { ChevronLeft } from "lucide-react";
import { LifecycleTimeline } from "@/components/lifecycle-timeline";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { RequestActions, RequestStatusBadge } from "@/components/request-actions";
import { fetchRequest } from "@/lib/api";
import { cn, formatDate } from "@/lib/utils";

export const metadata = { title: "Requests" };

function Info({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <dt className="text-xs uppercase tracking-wide text-muted-foreground">{label}</dt>
      <dd className={cn("mt-1 text-sm font-medium", mono && "font-mono break-all")}>{value || "-"}</dd>
    </div>
  );
}

export default async function RequestDetailPage({ params }: { params: Promise<{ name: string }> }) {
  const { name } = await params;
  const r = await fetchRequest(decodeURIComponent(name));
  if (!r) notFound();

  return (
    <div className="space-y-6">
      <Link href="/requests" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ChevronLeft className="size-4" />
        Requests
      </Link>

      <PageHeader title={r.name} description={`${r.environment} · ${r.kind} · ${r.provider}`}>
        <RequestStatusBadge status={r.status} />
        <RequestActions name={r.name} environment={r.environment} status={r.status} />
      </PageHeader>

      <LifecycleTimeline status={r.status} />

      <Card>
        <div className="border-b border-border px-5 py-4">
          <h2 className="text-sm font-semibold tracking-tight">Request context</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Governance and ownership data available from the current request API.
          </p>
        </div>
        <dl className="grid grid-cols-1 gap-5 p-5 sm:grid-cols-3">
          <Info label="Requester" value={r.requester} />
          <Info label="Kind" value={r.kind} />
          <Info label="Provider" value={r.provider} />
          <Info label="Blueprint" value={r.blueprint ?? ""} />
          <Info label="GLPI ticket" value={r.ticketRef ?? ""} mono />
          <Info label="Resource" value={r.resourceRef ?? ""} mono />
          <Info label="Decided by" value={r.decidedBy ?? ""} />
          <Info label="Reason" value={r.reason ?? ""} />
          <Info label="Created" value={formatDate(r.createdAt)} />
        </dl>
      </Card>
    </div>
  );
}
