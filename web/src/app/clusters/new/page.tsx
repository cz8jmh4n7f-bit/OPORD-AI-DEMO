import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { NewClusterForm } from "@/components/new-cluster-form";
import { fetchProviders } from "@/lib/api";

export const metadata = { title: "New cluster" };

export default async function NewClusterPage({
  searchParams,
}: {
  searchParams: Promise<{ provider?: string }>;
}) {
  const [providers, sp] = await Promise.all([fetchProviders(), searchParams]);
  return (
    <div className="max-w-3xl space-y-6">
      <Link
        href="/clusters"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ChevronLeft className="size-4" />
        Clusters
      </Link>
      <PageHeader
        title="New cluster"
        description="Create a Kubernetes cluster - OPORD reconciles infrastructure to match."
      />
      <NewClusterForm providers={providers} initialProvider={sp.provider} />
    </div>
  );
}
