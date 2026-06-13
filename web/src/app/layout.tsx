import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import { headers } from "next/headers";
import "./globals.css";
import { LayoutShell } from "@/components/layout/layout-shell";
import { AppProviders } from "@/components/providers";
import { checkApi } from "@/lib/api";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  // Per-route pages export `metadata.title` (e.g. "Clusters"); this template
  // composes them into "Clusters · OPORD" so browser tabs/history/bookmarks are
  // distinguishable. The default is used on routes that set no title.
  title: {
    default: "OPORD - Infrastructure Orchestration",
    template: "%s · OPORD",
  },
  description:
    "Declarative infrastructure operations: provision Kubernetes clusters across vSphere, Proxmox, and more.",
};

// Set the theme class before paint to avoid a flash of the wrong theme. (The AI
// workspace is route-scoped - see lib/ai-mode.ts - so no persisted mode here.)
const themeScript = `(function(){try{var t=localStorage.getItem('theme');var d=t?t==='dark':window.matchMedia('(prefers-color-scheme: dark)').matches;if(d){document.documentElement.classList.add('dark');}}catch(e){}})();`;

export default async function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  // Honest reachability check: when the API is down we show a banner rather than
  // letting every page render a misleading empty state.
  const apiOk = await checkApi();
  // CSP nonce minted per-request by middleware.ts; authorizes the inline script.
  const nonce = (await headers()).get("x-nonce") ?? undefined;

  return (
    <html
      lang="en"
      className={`${geistSans.variable} ${geistMono.variable} h-full`}
      suppressHydrationWarning
    >
      <head>
        <script nonce={nonce} dangerouslySetInnerHTML={{ __html: themeScript }} />
      </head>
      <body className="min-h-full">
        <AppProviders>
          <LayoutShell apiOk={apiOk}>{children}</LayoutShell>
        </AppProviders>
      </body>
    </html>
  );
}
