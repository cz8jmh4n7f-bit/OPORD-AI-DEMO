import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { NewCacheForm } from "@/components/new-cache-form";
import { fetchProviders } from "@/lib/api";

export const metadata = { title: "New cache" };

export default async function NewCachePage({
  searchParams,
}: {
  searchParams: Promise<{ provider?: string }>;
}) {
  const [providers, sp] = await Promise.all([fetchProviders(), searchParams]);
  return (
    <div className="max-w-3xl space-y-6">
      <Link href="/caches" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ChevronLeft className="size-4" />
        Caches
      </Link>
      <PageHeader title="New cache" description="A managed Redis cache, TLS-only. Access keys are never persisted by OPORD." />
      <NewCacheForm providers={providers} initialProvider={sp.provider} />
    </div>
  );
}
