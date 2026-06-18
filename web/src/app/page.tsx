import { redirect } from "next/navigation";

// OPORD is an AI service governance product; the root route lands in the AI
// workspace.
export default function Home() {
  redirect("/ai/overview");
}
