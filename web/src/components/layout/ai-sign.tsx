"use client";

import { useRouter } from "next/navigation";
import { useAIMode } from "@/lib/ai-mode";

// AINeonSign is the topbar switch between the two workspaces, styled like a
// motel sign: a dim, dead neon tube on the infrastructure side; a glowing orange
// tube inside the AI workspace. It is pure navigation - the lit state derives
// from the route, so the sign, the nav, and the page can never disagree.
export function AINeonSign() {
  const on = useAIMode();
  const router = useRouter();

  function toggle() {
    router.push(on ? "/" : "/ai/overview");
  }

  return (
    <button
      type="button"
      onClick={toggle}
      aria-pressed={on}
      aria-label={on ? "AI workspace - switch back to infrastructure" : "Switch to the AI workspace"}
      title={on ? "AI workspace - click for infrastructure" : "Open the AI workspace"}
      className={`neon-ai ${on ? "neon-ai-on" : "neon-ai-off"}`}
    >
      AI
    </button>
  );
}
