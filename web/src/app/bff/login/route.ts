import { cookies } from "next/headers";
import { NextResponse } from "next/server";

// Server-side login: the browser POSTs the pasted API key here; we validate it
// against the OPORD API (GET /api/v1/me) and, on success, set the key as an
// HttpOnly cookie (never readable by JS) plus a readable `opord_user` (just the
// email, for the sidebar). The /bff proxy then forwards the HttpOnly key on every
// browser request. Mirrors the old client-side storeAuth, but HttpOnly.
const BASE =
  process.env.OPORD_API_URL ?? process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

const MAX_AGE = 60 * 60 * 24 * 30; // 30 days

export async function POST(req: Request): Promise<NextResponse> {
  const { key } = (await req.json().catch(() => ({ key: "" }))) as { key?: string };
  const token = (key ?? "").trim();
  if (!token) return NextResponse.json({ error: "missing API key" }, { status: 400 });

  let me: Response;
  try {
    me = await fetch(`${BASE}/api/v1/me`, {
      headers: { authorization: `Bearer ${token}` },
      cache: "no-store",
    });
  } catch (err) {
    return NextResponse.json({ error: `could not reach the API: ${String(err)}` }, { status: 502 });
  }
  if (!me.ok) {
    return NextResponse.json(
      { error: me.status === 401 ? "invalid API key" : `login failed (${me.status})` },
      { status: me.status },
    );
  }
  const identity = (await me.json()) as { email: string; tenant: string; role: string };

  // Secure only over real HTTPS so the localhost http demo still works.
  const secure = new URL(req.url).protocol === "https:";
  const jar = await cookies();
  jar.set("opord_key", token, { httpOnly: true, secure, sameSite: "lax", path: "/", maxAge: MAX_AGE });
  jar.set("opord_user", identity.email ?? "", { httpOnly: false, secure, sameSite: "lax", path: "/", maxAge: MAX_AGE });

  return NextResponse.json(identity);
}
