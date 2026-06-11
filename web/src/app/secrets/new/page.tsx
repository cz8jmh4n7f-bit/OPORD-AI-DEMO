import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { NewSecretForm } from "@/components/new-secret-form";
import { fetchProviders } from "@/lib/api";

export const metadata = { title: "New secret" };

export default async function NewSecretPage({
  searchParams,
}: {
  searchParams: Promise<{ provider?: string }>;
}) {
  const [providers, sp] = await Promise.all([fetchProviders(), searchParams]);
  return (
    <div className="max-w-3xl space-y-6">
      <Link href="/secrets" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ChevronLeft className="size-4" />
        Secrets
      </Link>
      <PageHeader title="New secret" description="A managed secret container; values are set out-of-band so OPORD never holds plaintext." />
      <NewSecretForm providers={providers} initialProvider={sp.provider} />
    </div>
  );
}
