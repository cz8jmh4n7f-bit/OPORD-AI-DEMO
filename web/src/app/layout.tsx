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
  // Per-route pages export `metadata.title` (e.g. "Budgets"); this template
  // composes them into "Budgets · OPORD" so browser tabs/history/bookmarks are
  // distinguishable. The default is used on routes that set no title.
  title: {
    default: "OPORD - AI Service Governance",
    template: "%s · OPORD",
  },
  description:
    "Governed access to AI services: request, approve, meter, and audit, on the platform you already run.",
};

// Set the theme class before paint to avoid a flash. Dark is the DEFAULT
// (security command center); only an explicit "light" choice opts out. The
// catch also defaults to dark so a storage failure never flashes light.
const themeScript = `(function(){try{if(localStorage.getItem('theme')!=='light'){document.documentElement.classList.add('dark');}}catch(e){document.documentElement.classList.add('dark');}})();`;

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
