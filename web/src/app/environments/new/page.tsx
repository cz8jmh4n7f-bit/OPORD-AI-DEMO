import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { NewEnvironmentForm } from "@/components/new-environment-form";
import { fetchProviders, fetchBlueprints } from "@/lib/api";

export const metadata = { title: "New environment" };

export default async function NewEnvironmentPage({
  searchParams,
}: {
  searchParams: Promise<{ provider?: string }>;
}) {
  const [providers, blueprints, sp] = await Promise.all([
    fetchProviders(),
    fetchBlueprints(),
    searchParams,
  ]);

  return (
    <div className="max-w-3xl space-y-6">
      <Link
        href="/environments"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ChevronLeft className="size-4" />
        Environments
      </Link>

      <PageHeader title="New environment" description="Pick a blueprint and a backend - OPORD provisions the whole thing." />

      <NewEnvironmentForm providers={providers} blueprints={blueprints} initialProvider={sp.provider} />
    </div>
  );
}
