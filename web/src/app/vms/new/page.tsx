import Link from "next/link";
import { ChevronLeft, Info } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { NewVMForm } from "@/components/new-vm-form";
import { fetchProviders } from "@/lib/api";

export const metadata = { title: "New VM" };

export default async function NewVMPage({
  searchParams,
}: {
  searchParams: Promise<{ provider?: string }>;
}) {
  const [providers, sp] = await Promise.all([fetchProviders(), searchParams]);

  return (
    <div className="space-y-6">
      <Link
        href="/vms"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ChevronLeft className="size-4" />
        Virtual machines
      </Link>

      <PageHeader
        title="New virtual machine"
        description="Pick your compute and network - OPORD provisions it on the chosen provider."
      />

      <div className="flex max-w-3xl items-start gap-3 rounded-xl border border-info/30 bg-info/10 p-4 text-sm text-info">
        <Info className="mt-0.5 size-4 shrink-0" />
        <p>
          Submitting registers the VM and validates the spec. Live provisioning then runs in the
          background on the selected provider.
        </p>
      </div>

      <div className="max-w-3xl">
        <NewVMForm providers={providers} initialProvider={sp.provider} />
      </div>
    </div>
  );
}
