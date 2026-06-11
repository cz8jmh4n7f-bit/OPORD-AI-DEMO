"use client";

import { useRouter } from "next/navigation";
import { setAIMode, useAIMode } from "@/lib/ai-mode";

// AINeonSign is the topbar "AI" mode switch, styled like a motel sign: a dim,
// dead neon tube when off; a glowing orange tube when on. Clicking it enters or
// leaves the AI workspace (which filters the nav to AI-only) and routes to the
// matching home so you land where the menu now points.
export function AINeonSign() {
  const on = useAIMode();
  const router = useRouter();

  function toggle() {
    const next = !on;
    setAIMode(next);
    router.push(next ? "/ai/overview" : "/");
  }

  return (
    <button
      type="button"
      onClick={toggle}
      aria-pressed={on}
      aria-label={on ? "AI mode on - switch back to infrastructure" : "Switch to AI mode"}
      title={on ? "AI mode - click to exit" : "Enter AI mode"}
      className={`neon-ai ${on ? "neon-ai-on" : "neon-ai-off"}`}
    >
      AI
    </button>
  );
}
