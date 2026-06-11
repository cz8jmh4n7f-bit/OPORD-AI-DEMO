import { redirect } from "next/navigation";

// The AI workspace home is the overview dashboard. Bare /ai redirects there so the
// "AI" sign, the ComingSoon CTA, and a typed /ai all land on the same place.
export default function AIIndexPage() {
  redirect("/ai/overview");
}
