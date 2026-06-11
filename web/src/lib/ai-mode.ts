// AI-mode store - the topbar "AI" neon sign flips this. Mirrors the theme
// toggle: the source of truth lives in the DOM (`data-ai` on <html>), read as an
// external store rather than mirrored into React state via an effect. An inline
// script in the root layout applies the persisted value before paint (no flash),
// and a MutationObserver drives re-renders. Persisted to localStorage so the
// chosen workspace survives reloads.
"use client";

import { useSyncExternalStore } from "react";

function subscribe(onChange: () => void) {
  const observer = new MutationObserver(onChange);
  observer.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ["data-ai"],
  });
  return () => observer.disconnect();
}

function getSnapshot() {
  return document.documentElement.getAttribute("data-ai") === "on";
}

/** useAIMode returns whether the AI workspace mode is active. */
export function useAIMode(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, () => false);
}

/** setAIMode lights or dims the sign; the MutationObserver drives re-renders. */
export function setAIMode(on: boolean) {
  document.documentElement.setAttribute("data-ai", on ? "on" : "off");
  try {
    localStorage.setItem("ai-mode", on ? "on" : "off");
  } catch {
    /* ignore */
  }
}
