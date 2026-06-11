"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Loader2, Scaling } from "lucide-react";
import { button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { authHeaders } from "@/lib/client-auth";
import { useToast } from "@/components/ui/toast";
import { useConfirm } from "@/components/ui/confirm";

const API = "/bff";

// ScaleClusterButton changes a cluster's worker count (day-2). Prompts, then
// POSTs /clusters/{name}/scale; the cluster re-provisions in the background.
export function ScaleClusterButton({ name, environment, workers, status }: { name: string; environment: string; workers: number; status: string }) {
  const router = useRouter();
  const { toast } = useToast();
  const { prompt } = useConfirm();
  const [busy, setBusy] = useState(false);

  const gone = status === "destroying" || status === "destroyed";

  async function scale() {
    const input = await prompt({
      title: `Scale cluster “${name}”`,
      label: `Number of workers (currently ${workers})`,
      defaultValue: String(workers),
      confirmLabel: "Scale",
    });
    if (input == null) return;
    const next = parseInt(input, 10);
    if (!Number.isInteger(next) || next < 1) {
      toast({ variant: "error", title: "Invalid count", description: "Enter a whole number ≥ 1." });
      return;
    }
    if (next === workers) {
      // Same count = nothing to do. Tell the user so the click never feels silent.
      toast({ title: `Already at ${workers} workers`, description: "No change - pick a different count to scale." });
      return;
    }
    setBusy(true);
    try {
      const res = await fetch(
        `${API}/api/v1/clusters/${encodeURIComponent(name)}/scale?env=${encodeURIComponent(environment)}`,
        { method: "POST", headers: { "Content-Type": "application/json", ...authHeaders() }, body: JSON.stringify({ workers: next }) },
      );
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        toast({ variant: "error", title: "Scale failed", description: data.error ?? `Request failed (${res.status})` });
        return;
      }
      toast({ variant: "success", title: `Scaling “${name}” to ${next} workers`, description: "Re-provisioning in the background." });
      router.refresh();
    } catch (err) {
      toast({ variant: "error", title: "Scale failed", description: String(err) });
    } finally {
      setBusy(false);
    }
  }

  return (
    <button type="button" onClick={scale} disabled={busy || gone} className={cn(button({ variant: "outline", size: "sm" }))} title="Change worker count">
      {busy ? <Loader2 className="size-4 animate-spin" /> : <Scaling className="size-4" />}
      Scale
    </button>
  );
}
