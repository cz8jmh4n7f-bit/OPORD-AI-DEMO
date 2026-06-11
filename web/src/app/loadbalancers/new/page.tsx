import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { NewLoadBalancerForm } from "@/components/new-loadbalancer-form";
import { fetchProviders } from "@/lib/api";

export const metadata = { title: "New load balancer" };

export default async function NewLoadBalancerPage({
  searchParams,
}: {
  searchParams: Promise<{ provider?: string }>;
}) {
  const [providers, sp] = await Promise.all([fetchProviders(), searchParams]);
  return (
    <div className="max-w-3xl space-y-6">
      <Link href="/loadbalancers" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ChevronLeft className="size-4" />
        Load balancers
      </Link>
      <PageHeader title="New load balancer" description="An internet-facing or internal ALB fronting your instances, IPs, or a Lambda." />
      <NewLoadBalancerForm providers={providers} initialProvider={sp.provider} />
    </div>
  );
}
