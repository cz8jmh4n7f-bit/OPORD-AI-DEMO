"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Loader2, Ban } from "lucide-react";
import { authHeaders } from "@/lib/client-auth";
import { button } from "@/components/ui/button";
import { useConfirm } from "@/components/ui/confirm";
import { useToast } from "@/components/ui/toast";
import { cn } from "@/lib/utils";

const API = "/bff";

export function AIInstanceActions({ id, status }: { id: string; status: string }) {
  const router = useRouter();
  const { confirm } = useConfirm();
  const { toast } = useToast();
  const [busy, setBusy] = useState(false);

  if (!["active", "suspended"].includes(status)) {
    return <span className="text-xs text-muted-foreground">-</span>;
  }

  async function revoke() {
    const ok = await confirm({
      title: "Revoke AI access?",
      message: "OPORD marks this access instance revoked and audits it. Remember to remove the seat/key in the provider console too (manual in MVP).",
      confirmLabel: "Revoke",
      danger: true,
    });
    if (!ok) return;
    setBusy(true);
    try {
      const res = await fetch(`${API}/api/v1/ai/instances/${encodeURIComponent(id)}/revoke`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ by: "web" }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        toast({ variant: "error", title: "Revoke failed", description: data.error ?? `Request failed (${res.status})` });
        return;
      }
      toast({ variant: "success", title: "AI access revoked" });
      router.refresh();
    } catch (err) {
      toast({ variant: "error", title: "Revoke failed", description: String(err) });
    } finally {
      setBusy(false);
    }
  }

  return (
    <button type="button" onClick={revoke} disabled={busy} className={cn(button({ variant: "danger", size: "sm" }), busy && "opacity-70")}>
      {busy ? <Loader2 className="size-4 animate-spin" /> : <Ban className="size-4" />}
      Revoke
    </button>
  );
}
