"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Check, Loader2, X } from "lucide-react";
import { Badge, type BadgeVariant } from "@/components/ui/badge";
import { button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { authHeaders } from "@/lib/client-auth";
import { useToast } from "@/components/ui/toast";
import { useConfirm } from "@/components/ui/confirm";

const API = "/bff";

const statusVariant: Record<string, BadgeVariant> = {
  pending_approval: "warning",
  approved: "info",
  provisioning: "info",
  completed: "success",
  rejected: "default",
  failed: "danger",
};

export function RequestStatusBadge({ status }: { status: string }) {
  return <Badge variant={statusVariant[status] ?? "default"}>{status.replace("_", " ")}</Badge>;
}

// RequestActions shows Approve/Reject buttons for a pending request. Approving
// triggers provisioning; rejecting declines it. Both refresh the view.
export function RequestActions({ name, environment, status }: { name: string; environment: string; status: string }) {
  const router = useRouter();
  const { toast } = useToast();
  const { confirm } = useConfirm();
  const [busy, setBusy] = useState<"" | "approve" | "reject">("");

  if (status !== "pending_approval") {
    return <span className="text-xs text-muted-foreground">-</span>;
  }

  async function decide(action: "approve" | "reject") {
    if (action === "reject") {
      const ok = await confirm({
        title: `Reject request “${name}”?`,
        message: "The requester is notified; nothing is provisioned.",
        confirmLabel: "Reject",
        danger: true,
      });
      if (!ok) return;
    }
    setBusy(action);
    try {
      const res = await fetch(
        `${API}/api/v1/requests/${encodeURIComponent(name)}/${action}?env=${encodeURIComponent(environment)}`,
        { method: "POST", headers: { "Content-Type": "application/json", ...authHeaders() }, body: JSON.stringify({ by: "web" }) },
      );
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        toast({ variant: "error", title: `${action === "approve" ? "Approve" : "Reject"} failed`, description: data.error ?? `Request failed (${res.status})` });
        return;
      }
      toast({
        variant: "success",
        title: action === "approve" ? `Request “${name}” approved` : `Request “${name}” rejected`,
        description: action === "approve" ? "Provisioning started." : undefined,
      });
      router.refresh();
    } catch (err) {
      toast({ variant: "error", title: "Request failed", description: String(err) });
    } finally {
      setBusy("");
    }
  }

  return (
    <div className="flex items-center justify-end gap-2">
      <button type="button" onClick={() => decide("approve")} disabled={!!busy} className={cn(button({ size: "sm" }))}>
        {busy === "approve" ? <Loader2 className="size-4 animate-spin" /> : <Check className="size-4" />}
        Approve
      </button>
      <button type="button" onClick={() => decide("reject")} disabled={!!busy} className={cn(button({ variant: "danger", size: "sm" }))}>
        {busy === "reject" ? <Loader2 className="size-4 animate-spin" /> : <X className="size-4" />}
        Reject
      </button>
    </div>
  );
}
