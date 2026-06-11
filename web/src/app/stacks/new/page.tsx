import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { NewStackForm } from "@/components/new-stack-form";
import { fetchProviders } from "@/lib/api";

export const metadata = { title: "New stack" };

export default async function NewStackPage({
  searchParams,
}: {
  searchParams: Promise<{ provider?: string }>;
}) {
  const [providers, sp] = await Promise.all([fetchProviders(), searchParams]);

  return (
    <div className="max-w-3xl space-y-6">
      <Link
        href="/stacks"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ChevronLeft className="size-4" />
        Stacks
      </Link>

      <PageHeader title="New stack" description="Point at any OpenTofu module - OPORD runs it with managed state." />

      <NewStackForm providers={providers} initialProvider={sp.provider} />
    </div>
  );
}
