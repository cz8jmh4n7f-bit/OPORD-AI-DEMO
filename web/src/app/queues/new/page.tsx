import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { NewQueueForm } from "@/components/new-queue-form";
import { fetchProviders } from "@/lib/api";

export const metadata = { title: "New queue" };

export default async function NewQueuePage({
  searchParams,
}: {
  searchParams: Promise<{ provider?: string }>;
}) {
  const [providers, sp] = await Promise.all([fetchProviders(), searchParams]);
  return (
    <div className="max-w-3xl space-y-6">
      <Link href="/queues" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ChevronLeft className="size-4" />
        Queues
      </Link>
      <PageHeader title="New queue" description="A managed message queue with optional FIFO ordering and a dead-letter queue." />
      <NewQueueForm providers={providers} initialProvider={sp.provider} />
    </div>
  );
}
