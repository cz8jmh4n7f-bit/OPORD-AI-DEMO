import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { NewCertForm } from "@/components/new-cert-form";
import { fetchProviders } from "@/lib/api";

export const metadata = { title: "New certificate" };

export default async function NewCertPage({
  searchParams,
}: {
  searchParams: Promise<{ provider?: string }>;
}) {
  const [providers, sp] = await Promise.all([fetchProviders(), searchParams]);
  return (
    <div className="max-w-3xl space-y-6">
      <Link href="/certs" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ChevronLeft className="size-4" />
        Certificates
      </Link>
      <PageHeader title="New certificate" description="An ACM TLS certificate with automatic DNS validation via Route 53." />
      <NewCertForm providers={providers} initialProvider={sp.provider} />
    </div>
  );
}
