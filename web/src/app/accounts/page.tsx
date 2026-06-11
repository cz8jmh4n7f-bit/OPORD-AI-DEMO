import Link from "next/link";
import { Building2, Plus } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Card } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { EmptyState } from "@/components/empty-state";
import { ClusterStatusBadge } from "@/components/status-badge";
import { DestroyButton } from "@/components/destroy-button";
import { fetchAccounts } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

// Layer chip ordering. AWS: account/baseline/access/network/security.
// Azure (ADR-0009): subscription/baseline/key_vault/security_hardening/
// secure_vnet/rbac/policy. Unknown keys fall to the end (indexOf -1).
const LAYER_ORDER = [
  "account",
  "subscription",
  "baseline",
  "key_vault",
  "access",
  "rbac",
  "network",
  "secure_vnet",
  "security",
  "security_hardening",
  "policy",
];

export const metadata = { title: "Accounts" };

export default async function AccountsPage() {
  const accounts = await fetchAccounts();

  return (
    <div className="space-y-6">
      <PageHeader
        title="Accounts"
        description="Provisioned cloud landing zones: AWS member accounts, Azure managed subscriptions, and GCP projects, each with layered governance baselines."
      >
        <Link href="/accounts/new" className={button({ size: "sm" })}>
          <Plus className="size-4" />
          New account
        </Link>
      </PageHeader>

      {accounts.length === 0 ? (
        <Card>
          <EmptyState
            icon={Building2}
            title="No accounts yet"
            description="Provision a member account - OPORD creates it via Organizations, then applies the baseline, secure VPC (CIDR from the Vault pool), and security layers across accounts via AssumeRole."
            action={{ href: "/accounts/new", label: "New account" }}
          />
        </Card>
      ) : (
        <Card className="overflow-hidden p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
                  <th scope="col" className="px-5 py-3 font-medium">Name</th>
                  <th scope="col" className="px-5 py-3 font-medium">Provider</th>
                  <th scope="col" className="px-5 py-3 font-medium">Account ID</th>
                  <th scope="col" className="px-5 py-3 font-medium">Layers</th>
                  <th scope="col" className="px-5 py-3 font-medium">Status</th>
                  <th scope="col" className="px-5 py-3 font-medium">Created</th>
                  <th scope="col" className="sticky right-0 bg-card px-5 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {accounts.map((a) => {
                  const layers = a.layers ?? {};
                  const keys = Object.keys(layers).sort(
                    (x, y) => LAYER_ORDER.indexOf(x) - LAYER_ORDER.indexOf(y),
                  );
                  return (
                    <tr key={a.id} className="border-b border-border last:border-0 align-top hover:bg-muted/60">
                      <td className="px-5 py-3">
                        <span className="font-medium">{a.name}</span>
                        <div className="text-xs text-muted-foreground">
                          {a.csaId} · {a.cloudName}
                        </div>
                      </td>
                      <td className="px-5 py-3 text-muted-foreground">{a.provider}</td>
                      <td className="px-5 py-3 font-mono text-xs text-muted-foreground">{a.accountId || "-"}</td>
                      <td className="px-5 py-3">
                        {keys.length === 0 ? (
                          <span className="text-xs text-muted-foreground">-</span>
                        ) : (
                          <div className="flex flex-wrap gap-1">
                            {keys.map((k) => {
                              const ok = layers[k].startsWith("ready");
                              return (
                                <span
                                  key={k}
                                  title={layers[k]}
                                  className={`rounded-md px-1.5 py-0.5 text-[10px] font-medium ${
                                    ok ? "bg-success/10 text-success" : "bg-muted text-muted-foreground"
                                  }`}
                                >
                                  {k}
                                </span>
                              );
                            })}
                          </div>
                        )}
                      </td>
                      <td className="px-5 py-3">
                        <ClusterStatusBadge status={a.status} error={a.lastError} />
                      </td>
                      <td className="px-5 py-3 text-muted-foreground">{timeAgo(a.createdAt)}</td>
                      <td className="sticky right-0 bg-card px-5 py-3 text-right">
                        <DestroyButton resource="accounts" name={a.name} environment={a.environment} status={a.status} />
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </Card>
      )}
    </div>
  );
}
