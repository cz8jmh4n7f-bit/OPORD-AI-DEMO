import { CatalogBrowser } from "@/components/catalog/catalog-browser";
import { PageHeader } from "@/components/page-header";
import { fetchProviders } from "@/lib/api";

export const metadata = { title: "Catalog" };

export default async function CatalogPage() {
  const providers = await fetchProviders();

  return (
    <div className="space-y-6">
      <PageHeader
        title="Catalog"
        description="Request governed infrastructure services across public cloud, on-prem, and hybrid providers."
      />
      <CatalogBrowser providers={providers} />
    </div>
  );
}
