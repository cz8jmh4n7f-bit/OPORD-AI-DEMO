import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { NewS3Form } from "@/components/new-s3-form";
import { fetchProviders } from "@/lib/api";

export const metadata = { title: "New bucket" };

export default async function NewS3Page({
  searchParams,
}: {
  searchParams: Promise<{ provider?: string }>;
}) {
  const [providers, sp] = await Promise.all([fetchProviders(), searchParams]);
  return (
    <div className="max-w-3xl space-y-6">
      <Link href="/s3" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ChevronLeft className="size-4" />
        Object storage
      </Link>
      <PageHeader title="New S3 bucket" description="Private bucket with versioning and public access block enabled by default." />
      <NewS3Form providers={providers} initialProvider={sp.provider} />
    </div>
  );
}
