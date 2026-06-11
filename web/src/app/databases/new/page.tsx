import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { NewDatabaseForm } from "@/components/new-database-form";
import { fetchProviders } from "@/lib/api";

export const metadata = { title: "New database" };

export default async function NewDatabasePage({
  searchParams,
}: {
  searchParams: Promise<{ provider?: string }>;
}) {
  const [providers, sp] = await Promise.all([fetchProviders(), searchParams]);
  return (
    <div className="max-w-3xl space-y-6">
      <Link href="/databases" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ChevronLeft className="size-4" />
        Databases
      </Link>
      <PageHeader title="New database" description="Managed RDS instance - OPORD provisions it; RDS manages the master password." />
      <NewDatabaseForm providers={providers} initialProvider={sp.provider} />
    </div>
  );
}
