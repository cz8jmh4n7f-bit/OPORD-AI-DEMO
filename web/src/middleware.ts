import { NextRequest, NextResponse } from "next/server";

// Nonce-based Content-Security-Policy (defense-in-depth XSS hardening). A fresh
// per-request nonce authorizes our one inline script (the before-paint theme init
// in layout.tsx) and Next's own bootstrap; 'strict-dynamic' then lets that
// bootstrap load the rest of the JS chunks, so no host allowlist is needed for
// scripts. Because the browser now talks only to the same-origin /bff proxy (the
// HttpOnly-cookie BFF), connect-src can stay 'self' - no cross-origin API host.
//
// Dev-only relaxations: Next Fast Refresh needs 'unsafe-eval' and a ws: HMR
// socket. A production build (`next build`, what the demo ships) gets the strict
// policy with neither.
export function middleware(request: NextRequest) {
  const nonce = btoa(crypto.randomUUID());
  const dev = process.env.NODE_ENV === "development";

  const csp = [
    `default-src 'self'`,
    `script-src 'self' 'nonce-${nonce}' 'strict-dynamic'${dev ? " 'unsafe-eval'" : ""}`,
    `style-src 'self' 'unsafe-inline'`,
    `img-src 'self' data:`,
    `font-src 'self'`,
    `connect-src 'self'${dev ? " ws:" : ""}`,
    `frame-ancestors 'none'`,
    `base-uri 'self'`,
    `form-action 'self'`,
    `object-src 'none'`,
  ].join("; ");

  // Next reads the CSP from the REQUEST headers to apply the nonce to the scripts
  // it generates, so set it on both the forwarded request and the response.
  const requestHeaders = new Headers(request.headers);
  requestHeaders.set("x-nonce", nonce);
  requestHeaders.set("content-security-policy", csp);

  const res = NextResponse.next({ request: { headers: requestHeaders } });
  res.headers.set("content-security-policy", csp);
  return res;
}

export const config = {
  // Static assets don't need a per-document nonce; everything else (pages + the
  // /bff proxy) gets the policy.
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
