import { CornerDownRight } from "lucide-react";

/**
 * DeployTarget surfaces the OPORD-managed account a resource was deployed INTO
 * (a GCP project / Azure subscription / AWS member account, via `target_account`).
 * Renders a small badge under the provider name so the deploy location is visible
 * on every resource list; renders nothing when the resource lives in the
 * provider's default account.
 */
export function DeployTarget({ account }: { account?: string }) {
  if (!account) return null;
  return (
    <div
      className="mt-0.5 flex items-center gap-1 text-[11px] font-medium text-primary"
      title={`Deployed into managed account ${account}`}
    >
      <CornerDownRight className="size-3 shrink-0" />
      <span className="truncate font-mono">{account}</span>
    </div>
  );
}
