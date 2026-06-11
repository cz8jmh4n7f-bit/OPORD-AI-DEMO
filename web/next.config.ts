import type { NextConfig } from "next";

// Baseline security response headers, applied to every route by the Next server.
// These are universally safe (no risk of breaking hydration). A script-src CSP
// - the strongest XSS mitigation, and what would most reduce the impact of the
// JS-readable auth cookie - needs Next nonce wiring + per-page testing, so it is
// tracked as a follow-up alongside the HttpOnly BFF refactor.
const securityHeaders = [
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Strict-Transport-Security", value: "max-age=31536000; includeSubDomains" },
];

const nextConfig: NextConfig = {
  // Standalone output for Docker: `next build` emits .next/standalone with a
  // minimal server.js + only the node_modules it traces, so the runtime image
  // stays small. No effect on `next dev`.
  output: "standalone",
  async headers() {
    return [{ source: "/:path*", headers: securityHeaders }];
  },
};

export default nextConfig;
