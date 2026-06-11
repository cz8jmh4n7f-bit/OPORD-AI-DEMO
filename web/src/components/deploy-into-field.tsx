"use client";

import { useEffect, useState } from "react";
import { authHeaders } from "@/lib/client-auth";

const API = "/bff";

const inputCls =
  "h-9 w-full rounded-lg border border-input bg-card px-3 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";

type ManagedAccount = { id: string; name: string; provider: string; accountId?: string };

// DeployIntoField is the shared "Deploy into account/project/subscription" control
// (ADR-0013) used by every cloud create form. It fetches the account-factory's
// managed accounts once, filters them to the selected provider, and renders a
// dropdown (or a text fallback when none exist yet). Cloud-only: returns null for
// on-prem providers (vSphere/Proxmox). Controlled - the parent owns the value so it
// can put it on the spec at submit; the parent also resets the value on provider
// change. This replaces the block that used to be copy-pasted into 9 forms.
export function DeployIntoField({
  provider,
  providerType,
  value,
  onChange,
  label,
  hint,
}: {
  provider: string;
  providerType?: string;
  value: string;
  onChange: (v: string) => void;
  // Optional overrides so the same managed-account picker can be reused with
  // different semantics (e.g. "grant access to" vs the default "deploy into").
  label?: string;
  hint?: string;
}) {
  const [accounts, setAccounts] = useState<ManagedAccount[]>([]);

  useEffect(() => {
    void (async () => {
      try {
        const res = await fetch(`${API}/api/v1/accounts`, { headers: authHeaders() });
        if (res.ok) {
          const data = (await res.json()) as ManagedAccount[];
          setAccounts(Array.isArray(data) ? data : []);
        }
      } catch {
        /* ignore - the field falls back to a text input */
      }
    })();
  }, []);

  const isAws = providerType === "aws";
  const isAzure = providerType === "azure";
  const isGCP = providerType === "gcp";
  // Deploy-into only applies to clouds with an account/subscription/project factory.
  if (!(isAws || isAzure || isGCP)) return null;

  const managed = accounts.filter((a) => a.provider === provider && a.accountId);
  const noun = isAzure ? "subscription" : isAws ? "account" : "project";
  const defaultHint = isAws
    ? "A OPORD-managed AWS member account (from the account factory) to deploy INTO via cross-account AssumeRole; empty = the provider's own account."
    : `A OPORD-managed ${noun} (e.g. one the account factory created) to deploy INTO - using this provider's credentials. Empty = the provider's default.`;

  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-xs font-medium text-muted-foreground">{label ?? `Deploy into ${noun} (optional)`}</span>
      {managed.length > 0 ? (
        <select className={inputCls} value={value} onChange={(e) => onChange(e.target.value)}>
          <option value="">{`No managed ${noun} (default)`}</option>
          {managed.map((a) => (
            <option key={a.id} value={a.accountId ?? ""}>
              {/* The real cloud project/subscription/account id is what you deploy into
                  (the OPORD account name is just its prefix - showing both is redundant). */}
              {a.accountId}
            </option>
          ))}
        </select>
      ) : (
        <input
          className={inputCls}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={`No managed ${noun}s yet - leave blank for the provider default`}
        />
      )}
      <span className="text-[11px] text-muted-foreground">{hint ?? defaultHint}</span>
    </label>
  );
}
