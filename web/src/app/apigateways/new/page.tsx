import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { NewAPIGatewayForm } from "@/components/new-apigateway-form";
import { fetchProviders } from "@/lib/api";

export const metadata = { title: "New API gateway" };

export default async function NewAPIGatewayPage({
  searchParams,
}: {
  searchParams: Promise<{ provider?: string }>;
}) {
  const [providers, sp] = await Promise.all([fetchProviders(), searchParams]);
  return (
    <div className="max-w-3xl space-y-6">
      <Link href="/apigateways" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ChevronLeft className="size-4" />
        API gateways
      </Link>
      <PageHeader title="New API gateway" description="An HTTP API routing to a Lambda or an upstream HTTP service." />
      <NewAPIGatewayForm providers={providers} initialProvider={sp.provider} />
    </div>
  );
}
