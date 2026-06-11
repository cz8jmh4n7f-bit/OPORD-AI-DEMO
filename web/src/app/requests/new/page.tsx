import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { NewRequestForm } from "@/components/new-request-form";
import { fetchProviders } from "@/lib/api";

export const metadata = { title: "New request" };

export default async function NewRequestPage({
  searchParams,
}: {
  searchParams: Promise<{ provider?: string; kind?: string }>;
}) {
  const [providers, sp] = await Promise.all([fetchProviders(), searchParams]);

  return (
    <div className="max-w-3xl space-y-6">
      <Link href="/requests" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ChevronLeft className="size-4" />
        Requests
      </Link>

      <PageHeader title="New request" description="Submit a resource request for approval - it opens a ticket and waits." />

      <NewRequestForm providers={providers} initialProvider={sp.provider} initialKind={sp.kind} />
    </div>
  );
}
