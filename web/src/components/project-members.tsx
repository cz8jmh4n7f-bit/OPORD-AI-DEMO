"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Loader2, Plus, X } from "lucide-react";
import { button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { authHeaders } from "@/lib/client-auth";
import { useToast } from "@/components/ui/toast";

const API = "/bff";

// ProjectMembers shows the Identity Center group's members as chips and lets an
// operator add/remove them inline. Each change posts the full member list to
// /projects/{name}/members, which re-runs tofu apply to reconcile membership.
export function ProjectMembers({
  name,
  environment,
  members,
  status,
}: {
  name: string;
  environment: string;
  members: string[];
  status: string;
}) {
  const router = useRouter();
  const { toast } = useToast();
  const [value, setValue] = useState("");
  const [busy, setBusy] = useState(false);

  // While a project is being torn down (or gone) membership edits make no sense.
  const locked = status === "destroying" || status === "destroyed";

  async function submit(next: string[]) {
    setBusy(true);
    try {
      const res = await fetch(`${API}/api/v1/projects/${encodeURIComponent(name)}/members?env=${encodeURIComponent(environment)}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ members: next }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        toast({ variant: "error", title: "Update failed", description: data.error ?? `Request failed (${res.status})` });
        return;
      }
      toast({ variant: "success", title: "Members updated", description: "Re-provisioning in the background." });
      setValue("");
      router.refresh();
    } catch (err) {
      toast({ variant: "error", title: "Update failed", description: String(err) });
    } finally {
      setBusy(false);
    }
  }

  function add() {
    const v = value.trim();
    if (!v || members.includes(v)) {
      setValue("");
      return;
    }
    void submit([...members, v]);
  }

  function remove(m: string) {
    void submit(members.filter((x) => x !== m));
  }

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap gap-1.5">
        {members.length === 0 && <span className="text-xs text-muted-foreground">No members</span>}
        {members.map((m) => (
          <span
            key={m}
            className="inline-flex items-center gap-1 rounded-md bg-muted px-2 py-0.5 text-xs text-foreground"
          >
            {m}
            {!locked && (
              <button
                type="button"
                onClick={() => remove(m)}
                disabled={busy}
                className="text-muted-foreground hover:text-danger"
                title={`Remove ${m}`}
              >
                <X className="size-3" />
              </button>
            )}
          </span>
        ))}
      </div>
      {!locked && (
        <div className="flex items-center gap-1.5">
          <input
            value={value}
            onChange={(e) => setValue(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                add();
              }
            }}
            placeholder="username to add"
            disabled={busy}
            className="h-7 w-44 rounded-md border border-border bg-card px-2 text-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          />
          <button
            type="button"
            onClick={add}
            disabled={busy || value.trim() === ""}
            className={cn(button({ variant: "outline", size: "sm" }), "h-7 px-2")}
            title="Add member"
          >
            {busy ? <Loader2 className="size-3.5 animate-spin" /> : <Plus className="size-3.5" />}
            Add
          </button>
        </div>
      )}
    </div>
  );
}
