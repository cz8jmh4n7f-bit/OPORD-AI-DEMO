import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { NewTableForm } from "@/components/new-table-form";
import { fetchProviders } from "@/lib/api";

export const metadata = { title: "New table" };

export default async function NewTablePage({
  searchParams,
}: {
  searchParams: Promise<{ provider?: string }>;
}) {
  const [providers, sp] = await Promise.all([fetchProviders(), searchParams]);
  return (
    <div className="max-w-3xl space-y-6">
      <Link href="/tables" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ChevronLeft className="size-4" />
        Tables
      </Link>
      <PageHeader title="New table" description="Managed DynamoDB table - on-demand or provisioned capacity." />
      <NewTableForm providers={providers} initialProvider={sp.provider} />
    </div>
  );
}
