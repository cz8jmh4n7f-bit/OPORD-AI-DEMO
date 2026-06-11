// Client-side auth helpers. The API key now lives in an HttpOnly `opord_key`
// cookie that is set server-side by /bff/login and forwarded by the /bff proxy -
// it is NOT readable from JS. Browser requests go same-origin through /bff, so the
// cookie rides along automatically; no Authorization header is built client-side.
//
// authHeaders() is kept as a no-op so the many mutation components keep compiling
// unchanged (they spread it into fetch headers). SameSite=Lax on the cookie blocks
// cross-site POSTs, so routing mutations through the cookie does not open CSRF.

export function authHeaders(): Record<string, string> {
  return {};
}

// The readable identity (email only, never the key) for the sidebar. Set alongside
// the HttpOnly key by /bff/login; absent when not signed in.
export function getStoredUser(): string {
  if (typeof document === "undefined") return "";
  const m = document.cookie.match(/(?:^|;\s*)opord_user=([^;]+)/);
  return m ? decodeURIComponent(m[1]) : "";
}

// logoutRequest clears both cookies server-side (the key cookie is HttpOnly, so
// client JS cannot delete it directly).
export async function logoutRequest(): Promise<void> {
  try {
    await fetch("/bff/logout", { method: "POST" });
  } catch {
    // best-effort; the redirect still happens
  }
}
