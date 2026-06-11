"use client";

import { createContext, useCallback, useContext, useState, useSyncExternalStore, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { Check, Info, TriangleAlert, X } from "lucide-react";
import { cn } from "@/lib/utils";

type Variant = "success" | "error" | "info";
type Toast = { id: number; title: string; description?: string; variant: Variant };

// description is typed `unknown` because call sites often pass `data.error`
// straight from an `any`-typed res.json(). A relayed upstream error can be an
// OBJECT ({error:{message}}), and rendering an object as a React child throws
// in a loop that can take the whole tab down - so we coerce to a string here.
type ToastInput = { title: string; description?: unknown; variant?: Variant };
type Ctx = { toast: (t: ToastInput) => void };

function coerceDescription(d: unknown): string | undefined {
  if (d == null) return undefined;
  if (typeof d === "string") return d;
  if (d instanceof Error) return d.message;
  if (typeof d === "object") {
    const o = d as { message?: unknown; error?: unknown };
    if (typeof o.message === "string") return o.message;
    if (typeof o.error === "string") return o.error;
    try {
      return JSON.stringify(d);
    } catch {
      return String(d);
    }
  }
  return String(d);
}

const ToastCtx = createContext<Ctx | null>(null);

// useToast returns toast({ title, description?, variant? }). Replaces window.alert
// with non-blocking, themed notifications.
export function useToast(): Ctx {
  const c = useContext(ToastCtx);
  if (!c) throw new Error("useToast must be used within <ToastProvider>");
  return c;
}

let seq = 0;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const remove = useCallback((id: number) => {
    setToasts((cur) => cur.filter((t) => t.id !== id));
  }, []);

  const toast = useCallback(
    (t: ToastInput) => {
      const id = ++seq;
      setToasts((cur) => [...cur, { id, variant: "info", ...t, description: coerceDescription(t.description) }]);
      setTimeout(() => remove(id), 5000);
    },
    [remove],
  );

  return (
    <ToastCtx.Provider value={{ toast }}>
      {children}
      <Toaster toasts={toasts} onClose={remove} />
    </ToastCtx.Provider>
  );
}

const accent: Record<Variant, string> = {
  success: "text-success",
  error: "text-danger",
  info: "text-info",
};
const icon: Record<Variant, typeof Check> = {
  success: Check,
  error: TriangleAlert,
  info: Info,
};

// Mount detection without setState-in-effect: returns false on the server and
// the first client (hydration) render, true afterwards. Avoids hydrating a
// portal the server didn't render (the cause of the hydration mismatch).
const emptySubscribe = () => () => {};
function useMounted() {
  return useSyncExternalStore(
    emptySubscribe,
    () => true,
    () => false,
  );
}

// Portaled to <body> so the topbar's backdrop-blur (a containing block) can't
// trap the fixed positioning. Renders nothing until mounted to match SSR.
function Toaster({ toasts, onClose }: { toasts: Toast[]; onClose: (id: number) => void }) {
  const mounted = useMounted();
  if (!mounted) return null;
  return createPortal(
    <div
      // Persistent aria-live region (always in the DOM once mounted) so screen
      // readers announce toasts as they're inserted - WCAG 4.1.3. Errors use
      // role="alert" (assertive) so failures interrupt; success/info are polite.
      aria-live="polite"
      aria-atomic="false"
      className="pointer-events-none fixed bottom-4 right-4 z-[60] flex w-[min(92vw,22rem)] flex-col gap-2"
    >
      {toasts.map((t) => {
        const Icon = icon[t.variant];
        return (
          <div
            key={t.id}
            role={t.variant === "error" ? "alert" : "status"}
            className="pointer-events-auto flex items-start gap-3 rounded-xl border border-border bg-card p-3 shadow-lg"
          >
            <Icon className={cn("mt-0.5 size-4 shrink-0", accent[t.variant])} />
            <div className="min-w-0 flex-1">
              <div className="text-sm font-medium text-foreground">{t.title}</div>
              {t.description && <div className="mt-0.5 text-xs text-muted-foreground">{t.description}</div>}
            </div>
            <button
              type="button"
              onClick={() => onClose(t.id)}
              aria-label="Dismiss"
              className="text-muted-foreground transition-colors hover:text-foreground"
            >
              <X className="size-4" />
            </button>
          </div>
        );
      })}
    </div>,
    document.body,
  );
}
