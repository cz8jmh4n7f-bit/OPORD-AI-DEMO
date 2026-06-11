"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Loader2, Trash2, X } from "lucide-react";
import { button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { authHeaders } from "@/lib/client-auth";
import { useToast } from "@/components/ui/toast";
import { useConfirm } from "@/components/ui/confirm";

const API = "/bff";

// DestroyButton kicks off a `tofu destroy` for one resource (a VM or a cluster).
// The API returns immediately (202) and the teardown runs in the background, so
// we just refresh to pick up the new "destroying" status.
export function DestroyButton({
  resource,
  name,
  environment,
  status,
  size = "sm",
}: {
  resource: "vms" | "clusters" | "environments" | "stacks" | "databases" | "s3" | "secrets" | "queues" | "caches" | "tables" | "functions" | "projects" | "accounts" | "dns" | "certs" | "loadbalancers" | "apigateways" | "cdns";
  name: string;
  environment: string;
  status: string;
  size?: "sm" | "md";
}) {
  const router = useRouter();
  const { toast } = useToast();
  const { confirm } = useConfirm();
  const [busy, setBusy] = useState(false);

  const nouns: Record<string, string> = { clusters: "cluster", environments: "environment", stacks: "stack", vms: "VM", databases: "database", s3: "S3 bucket", secrets: "secret", queues: "queue", caches: "cache", tables: "table", functions: "function", projects: "project", accounts: "account", dns: "DNS zone", certs: "certificate", loadbalancers: "load balancer", apigateways: "API gateway", cdns: "CDN" };
  const noun = nouns[resource] ?? "resource";

  // A destroyed resource has no infra left, so offer "Remove" (forget the
  // tracking row) instead of a permanently-disabled Destroy. Supported for
  // vms/stacks/databases/clusters (all have a ?purge=true endpoint).
  const purgeable = ["vms", "stacks", "databases", "clusters", "s3", "secrets", "queues", "caches", "tables", "functions", "projects", "accounts", "dns", "certs", "loadbalancers", "apigateways", "cdns"].includes(resource) && status === "destroyed";
  const gone = status === "destroying" || status === "destroyed";

  async function run(url: string, msg: { okTitle: string; okDesc: string; failTitle: string }) {
    setBusy(true);
    try {
      const res = await fetch(url, { method: "DELETE", headers: authHeaders() });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        toast({ variant: "error", title: msg.failTitle, description: data.error ?? `Request failed (${res.status})` });
        return;
      }
      toast({ variant: "success", title: msg.okTitle, description: msg.okDesc });
      router.refresh();
    } catch (err) {
      toast({ variant: "error", title: msg.failTitle, description: String(err) });
    } finally {
      setBusy(false);
    }
  }

  async function destroy() {
    const ok = await confirm({
      title: `Destroy ${noun} “${name}”?`,
      message: `Environment: ${environment}. This runs tofu destroy and cannot be undone.`,
      confirmLabel: "Destroy",
      danger: true,
    });
    if (!ok) return;
    await run(`${API}/api/v1/${resource}/${encodeURIComponent(name)}?env=${encodeURIComponent(environment)}`, {
      okTitle: `Destroying ${noun} “${name}”`,
      okDesc: "Teardown runs in the background.",
      failTitle: "Destroy failed",
    });
  }

  async function purge() {
    const ok = await confirm({
      title: `Remove ${noun} “${name}” from the list?`,
      message: `Environment: ${environment}. The infrastructure is already destroyed - this only forgets the record and won't touch any cloud resources.`,
      confirmLabel: "Remove",
      danger: true,
    });
    if (!ok) return;
    await run(`${API}/api/v1/${resource}/${encodeURIComponent(name)}?env=${encodeURIComponent(environment)}&purge=true`, {
      okTitle: `Removed “${name}”`,
      okDesc: "The record is gone.",
      failTitle: "Remove failed",
    });
  }

  if (purgeable) {
    return (
      <button
        type="button"
        onClick={purge}
        disabled={busy}
        className={cn(button({ variant: "outline", size }))}
        title="Remove this VM from the list (infrastructure already destroyed)"
      >
        {busy ? <Loader2 className="size-4 animate-spin" /> : <X className="size-4" />}
        Remove
      </button>
    );
  }

  return (
    <button
      type="button"
      onClick={destroy}
      disabled={busy || gone}
      className={cn(button({ variant: "danger", size }))}
      title={gone ? "Already being torn down" : `Destroy this ${noun}`}
    >
      {busy ? <Loader2 className="size-4 animate-spin" /> : <Trash2 className="size-4" />}
      Destroy
    </button>
  );
}
