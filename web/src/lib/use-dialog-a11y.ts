"use client";

import { useEffect, useRef } from "react";

// useDialogA11y wires the accessibility basics for a portaled modal dialog:
// Escape-to-close, initial focus on the first control, focus trapping (Tab /
// Shift+Tab cycle within the dialog), and focus restoration to the trigger on
// close. Attach the returned ref to the dialog container element.
export function useDialogA11y(open: boolean, onClose: () => void) {
  const ref = useRef<HTMLDivElement>(null);
  // Callers pass an inline closure for onClose, so its identity changes every
  // render. Keep the latest in a ref and depend only on `open` - otherwise the
  // main effect re-runs on EVERY keystroke (state change -> render -> new
  // onClose), and the initial-focus call below yanks focus back to the first
  // input mid-typing.
  const onCloseRef = useRef(onClose);
  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  useEffect(() => {
    if (!open) return;
    const previouslyFocused = document.activeElement as HTMLElement | null;
    const container = ref.current;

    const focusable = () =>
      Array.from(
        container?.querySelectorAll<HTMLElement>(
          'a[href],button:not([disabled]),textarea:not([disabled]),input:not([disabled]),select:not([disabled]),[tabindex]:not([tabindex="-1"])',
        ) ?? [],
      ).filter((el) => el.offsetParent !== null);

    focusable()[0]?.focus();

    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault();
        onCloseRef.current();
        return;
      }
      if (e.key !== "Tab") return;
      const items = focusable();
      if (items.length === 0) return;
      const first = items[0];
      const last = items[items.length - 1];
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    }

    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("keydown", onKey);
      previouslyFocused?.focus?.();
    };
  }, [open]);

  return ref;
}
