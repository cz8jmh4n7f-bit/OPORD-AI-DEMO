"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { KeyRound, Loader2, TriangleAlert } from "lucide-react";
import { LogoMark } from "@/components/logo";
import { Card, CardContent } from "@/components/ui/card";
import { button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const API = "/bff";

// Login POSTs the pasted key to the same-origin /bff/login route, which validates
// it against the OPORD API and, on success, sets the key as an HttpOnly cookie
// (never readable by JS) + a readable identity. The /bff proxy then forwards the
// key on every request. No password is entered here - keys are minted by an admin
// via `opord user add`.
export default function LoginPage() {
  const router = useRouter();
  const [key, setKey] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const token = key.trim();
    if (!token) return;
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`${API}/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ key: token }),
      });
      if (!res.ok) {
        const data = (await res.json().catch(() => ({}))) as { error?: string };
        setError(
          res.status === 401 ? "Invalid API key." : data.error ?? `Login failed (${res.status}).`,
        );
        return;
      }
      router.push("/");
      router.refresh();
    } catch (err) {
      setError(`Could not reach the API: ${String(err)}`);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mx-auto flex min-h-[70vh] max-w-md flex-col justify-center">
      <div className="mb-8 flex items-center justify-center gap-2.5">
        <LogoMark className="h-7 text-foreground" />
        <span className="text-2xl font-bold tracking-tight text-foreground">OPORD</span>
      </div>

      <Card>
        <CardContent className="space-y-5 p-6">
          <div className="space-y-1">
            <h1 className="text-lg font-semibold tracking-tight">Sign in</h1>
            <p className="text-sm text-muted-foreground">
              Paste your OPORD API key. An admin can mint one with{" "}
              <code className="rounded bg-muted px-1 py-0.5 text-xs">opord user add</code>.
            </p>
          </div>

          {error && (
            <div className="flex items-start gap-2 rounded-xl border border-danger/30 bg-danger/10 p-3 text-sm text-danger">
              <TriangleAlert className="mt-0.5 size-4 shrink-0" />
              <span>{error}</span>
            </div>
          )}

          <form onSubmit={submit} className="space-y-4">
            <label className="flex flex-col gap-1.5">
              <span className="text-xs font-medium text-muted-foreground">API key</span>
              <div className="relative">
                <KeyRound className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <input
                  type="password"
                  autoFocus
                  autoComplete="off"
                  value={key}
                  onChange={(e) => setKey(e.target.value)}
                  placeholder="opd_..."
                  className="h-9 w-full rounded-lg border border-border bg-card pl-9 pr-3 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                />
              </div>
            </label>

            <button type="submit" disabled={busy || !key.trim()} className={cn(button({ size: "md" }), "w-full")}>
              {busy && <Loader2 className="size-4 animate-spin" />}
              Sign in
            </button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
