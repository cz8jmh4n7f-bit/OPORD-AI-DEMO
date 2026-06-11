import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { NewFunctionForm } from "@/components/new-function-form";
import { fetchProviders } from "@/lib/api";

export const metadata = { title: "New function" };

export default async function NewFunctionPage({
  searchParams,
}: {
  searchParams: Promise<{ provider?: string }>;
}) {
  const [providers, sp] = await Promise.all([fetchProviders(), searchParams]);
  return (
    <div className="max-w-3xl space-y-6">
      <Link href="/functions" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ChevronLeft className="size-4" />
        Functions
      </Link>
      <PageHeader title="New function" description="AWS Lambda - with no code supplied, OPORD ships a built-in handler so it's immediately invokable." />
      <NewFunctionForm providers={providers} initialProvider={sp.provider} />
    </div>
  );
}
