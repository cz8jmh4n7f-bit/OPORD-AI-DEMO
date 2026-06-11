import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { NewCDNForm } from "@/components/new-cdn-form";
import { fetchProviders } from "@/lib/api";

export const metadata = { title: "New CDN" };

export default async function NewCDNPage({
  searchParams,
}: {
  searchParams: Promise<{ provider?: string }>;
}) {
  const [providers, sp] = await Promise.all([fetchProviders(), searchParams]);
  return (
    <div className="max-w-3xl space-y-6">
      <Link href="/cdns" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ChevronLeft className="size-4" />
        CDNs
      </Link>
      <PageHeader title="New CDN" description="A CloudFront distribution fronting an S3 bucket, ALB, API Gateway, or custom origin." />
      <NewCDNForm providers={providers} initialProvider={sp.provider} />
    </div>
  );
}
