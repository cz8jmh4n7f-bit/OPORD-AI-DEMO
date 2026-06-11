"use client";

import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { button } from "./button";
import { cn } from "@/lib/utils";

type BaseOpts = {
  title: string;
  message?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
};
type PromptOpts = BaseOpts & {
  label?: string;
  placeholder?: string;
  defaultValue?: string;
  // When set, the confirm button stays disabled until the typed value matches
  // exactly (type-to-confirm guard for destructive actions).
  requireValue?: string;
};

type Ctx = {
  confirm: (o: BaseOpts) => Promise<boolean>;
  prompt: (o: PromptOpts) => Promise<string | null>;
};

const ConfirmCtx = createContext<Ctx | null>(null);

// useConfirm replaces window.confirm / window.prompt with themed, accessible
// dialogs: `await confirm({...})` returns boolean, `await prompt({...})` returns string|null.
export function useConfirm(): Ctx {
  const c = useContext(ConfirmCtx);
  if (!c) throw new Error("useConfirm must be used within <ConfirmProvider>");
  return c;
}

type State =
  | { open: false }
  | { open: true; mode: "confirm"; opts: BaseOpts }
  | { open: true; mode: "prompt"; opts: PromptOpts; value: string };

const inputCls =
  "h-9 w-full rounded-lg border border-input bg-card px-3 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";

export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<State>({ open: false });
  const resolver = useRef<((v: boolean | string | null) => void) | null>(null);
  const panelRef = useRef<HTMLDivElement>(null);

  const settle = useCallback((result: boolean | string | null) => {
    resolver.current?.(result);
    resolver.current = null;
    setState({ open: false });
  }, []);

  const confirm = useCallback(
    (o: BaseOpts) =>
      new Promise<boolean>((res) => {
        resolver.current = res as (v: boolean | string | null) => void;
        setState({ open: true, mode: "confirm", opts: o });
      }),
    [],
  );

  const prompt = useCallback(
    (o: PromptOpts) =>
      new Promise<string | null>((res) => {
        resolver.current = res as (v: boolean | string | null) => void;
        setState({ open: true, mode: "prompt", opts: o, value: o.defaultValue ?? "" });
      }),
    [],
  );

  useEffect(() => {
    if (!state.open) return;
    const previouslyFocused = document.activeElement as HTMLElement | null;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        settle(state.mode === "prompt" ? null : false);
        return;
      }
      // Trap Tab focus inside the dialog (WCAG 2.4.3 - honor the aria-modal contract).
      if (e.key === "Tab" && panelRef.current) {
        const f = panelRef.current.querySelectorAll<HTMLElement>(
          'a[href], button:not([disabled]), input, select, textarea, [tabindex]:not([tabindex="-1"])',
        );
        if (f.length === 0) return;
        const first = f[0];
        const last = f[f.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
    };
    document.addEventListener("keydown", onKey);
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = "";
      previouslyFocused?.focus?.(); // restore focus to the trigger on close
    };
  }, [state, settle]);

  return (
    <ConfirmCtx.Provider value={{ confirm, prompt }}>
      {children}
      {state.open &&
        typeof document !== "undefined" &&
        createPortal(
          <div
            className="fixed inset-0 z-[70] flex items-center justify-center p-4"
            role="dialog"
            aria-modal="true"
            aria-labelledby="confirm-dialog-title"
            aria-describedby={state.opts.message ? "confirm-dialog-desc" : undefined}
          >
            <div
              className="absolute inset-0 bg-black/50"
              onClick={() => settle(state.mode === "prompt" ? null : false)}
            />
            <div ref={panelRef} className="relative w-full max-w-md rounded-xl border border-border bg-card p-5 shadow-xl">
              <h2 id="confirm-dialog-title" className="text-base font-semibold text-foreground">
                {state.opts.title}
              </h2>
              {state.opts.message && (
                <p id="confirm-dialog-desc" className="mt-2 whitespace-pre-line text-sm text-muted-foreground">
                  {state.opts.message}
                </p>
              )}

              {state.mode === "prompt" && (
                <label className="mt-4 flex flex-col gap-1.5">
                  {state.opts.label && (
                    <span className="text-xs font-medium text-muted-foreground">{state.opts.label}</span>
                  )}
                  <input
                    className={inputCls}
                    value={state.value}
                    placeholder={state.opts.placeholder}
                    autoFocus
                    onChange={(e) => setState({ ...state, value: e.target.value })}
                    onKeyDown={(e) => {
                      if (e.key !== "Enter") return;
                      if (state.opts.requireValue != null && state.value !== state.opts.requireValue) return;
                      settle(state.value);
                    }}
                  />
                </label>
              )}

              <div className="mt-5 flex justify-end gap-2">
                <button
                  type="button"
                  onClick={() => settle(state.mode === "prompt" ? null : false)}
                  className={cn(button({ variant: "outline", size: "md" }))}
                >
                  {state.opts.cancelLabel ?? "Cancel"}
                </button>
                <button
                  type="button"
                  autoFocus={state.mode === "confirm"}
                  disabled={state.mode === "prompt" && state.opts.requireValue != null && state.value !== state.opts.requireValue}
                  onClick={() => settle(state.mode === "prompt" ? state.value : true)}
                  className={cn(button({ variant: state.opts.danger ? "danger" : "primary", size: "md" }))}
                >
                  {state.opts.confirmLabel ?? "Confirm"}
                </button>
              </div>
            </div>
          </div>,
          document.body,
        )}
    </ConfirmCtx.Provider>
  );
}
