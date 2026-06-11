import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { NewProjectForm } from "@/components/new-project-form";
import { fetchProviders } from "@/lib/api";

export const metadata = { title: "New project" };

export default async function NewProjectPage({
  searchParams,
}: {
  searchParams: Promise<{ provider?: string }>;
}) {
  const [providers, sp] = await Promise.all([fetchProviders(), searchParams]);
  return (
    <div className="max-w-3xl space-y-6">
      <Link href="/projects" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ChevronLeft className="size-4" />
        Projects
      </Link>
      <PageHeader title="New project" description="Grant a team access to an existing account via IAM Identity Center (group, permission set, and assignment)." />
      <NewProjectForm providers={providers} initialProvider={sp.provider} />
    </div>
  );
}
