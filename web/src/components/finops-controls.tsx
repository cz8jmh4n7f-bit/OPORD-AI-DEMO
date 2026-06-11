"use client";

import { useRouter, usePathname } from "next/navigation";

// Shared select styling, matching every create form in the app.
const inputCls =
  "h-9 w-full rounded-lg border border-input bg-card px-3 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";

// FinOpsControls is the secondary FinOps filter: the trailing window always, and -
// only once a cloud tile is selected - an account dropdown scoped to that cloud's
// accounts. Both preserve the selected cloud (?provider=) in the URL.
export function FinOpsControls({
  provider,
  accounts,
  account,
  days,
}: {
  provider: string;
  accounts: { id: string; name?: string }[];
  account: string;
  days: number;
}) {
  const router = useRouter();
  const pathname = usePathname();

  function push(nextAccount: string, nextDays: number) {
    const params = new URLSearchParams();
    if (provider) params.set("provider", provider);
    if (nextAccount) params.set("account", nextAccount);
    params.set("days", String(nextDays));
    router.push(`${pathname}?${params.toString()}`);
  }

  return (
    <div className="flex flex-wrap items-end gap-3">
      {provider && accounts.length > 0 && (
        <label className="flex flex-col gap-1.5">
          <span className="text-xs font-medium text-muted-foreground">Account</span>
          <select
            className={inputCls}
            value={account}
            onChange={(e) => push(e.target.value, days)}
            aria-label="Filter actuals by linked account"
          >
            <option value="">All accounts</option>
            {accounts.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name && a.name !== a.id ? `${a.name} - ${a.id}` : a.id}
              </option>
            ))}
          </select>
        </label>
      )}
      <label className="flex flex-col gap-1.5">
        <span className="text-xs font-medium text-muted-foreground">Window</span>
        <select
          className={inputCls}
          value={days}
          onChange={(e) => push(account, Number(e.target.value))}
          aria-label="Actuals trailing window"
        >
          <option value={7}>Last 7 days</option>
          <option value={30}>Last 30 days</option>
          <option value={90}>Last 90 days</option>
        </select>
      </label>
    </div>
  );
}
