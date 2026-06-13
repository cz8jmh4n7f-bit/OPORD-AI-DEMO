import { redirect } from "next/navigation";

// The AI workspace home is the overview dashboard. Bare /ai redirects there so the
// topbar "AI" sign and a typed /ai land on the same place.
export default function AIIndexPage() {
  redirect("/ai/overview");
}
