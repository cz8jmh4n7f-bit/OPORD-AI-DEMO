import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { NewAccountForm } from "@/components/new-account-form";
import { fetchProviders } from "@/lib/api";

export const metadata = { title: "New account" };

export default async function NewAccountPage({
  searchParams,
}: {
  searchParams: Promise<{ provider?: string }>;
}) {
  const [providers, sp] = await Promise.all([fetchProviders(), searchParams]);
  return (
    <div className="max-w-3xl space-y-6">
      <Link href="/accounts" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ChevronLeft className="size-4" />
        Accounts
      </Link>
      <PageHeader title="New account" description="Provision a governed landing zone - an AWS member account, an Azure subscription, or a GCP project - each with its baseline (identity, network, logging, policy)." />
      <NewAccountForm providers={providers} initialProvider={sp.provider} />
    </div>
  );
}
