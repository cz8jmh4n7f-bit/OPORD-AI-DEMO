import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { NewDNSForm } from "@/components/new-dns-form";
import { fetchProviders } from "@/lib/api";

export const metadata = { title: "New DNS zone" };

export default async function NewDNSPage({
  searchParams,
}: {
  searchParams: Promise<{ provider?: string }>;
}) {
  const [providers, sp] = await Promise.all([fetchProviders(), searchParams]);
  return (
    <div className="max-w-3xl space-y-6">
      <Link href="/dns" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ChevronLeft className="size-4" />
        DNS zones
      </Link>
      <PageHeader title="New DNS zone" description="A Route 53 hosted zone for your domain - public, or private to a VPC." />
      <NewDNSForm providers={providers} initialProvider={sp.provider} />
    </div>
  );
}
