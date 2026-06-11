import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";

// Same-origin BFF proxy. The browser calls /bff/api/v1/... (same origin as the
// console), this handler reads the HttpOnly `opord_key` cookie SERVER-SIDE and
// forwards the request to the OPORD API with a Bearer token. The key therefore
// never enters client JS, so the auth cookie can be HttpOnly. Page reads still
// talk to the API directly server-side (lib/api.ts); only browser-initiated
// mutations/GETs route through here.
const BASE =
  process.env.OPORD_API_URL ?? process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

async function forward(req: NextRequest, path: string[]): Promise<NextResponse> {
  const key = (await cookies()).get("opord_key")?.value;
  const url = `${BASE}/${path.join("/")}${req.nextUrl.search}`;

  const headers: Record<string, string> = {};
  const ct = req.headers.get("content-type");
  if (ct) headers["content-type"] = ct;
  if (key) headers["authorization"] = `Bearer ${key}`;

  const hasBody = req.method !== "GET" && req.method !== "HEAD";
  let upstream: Response;
  try {
    upstream = await fetch(url, {
      method: req.method,
      headers,
      body: hasBody ? await req.text() : undefined,
      cache: "no-store",
    });
  } catch (err) {
    return NextResponse.json({ error: `upstream unreachable: ${String(err)}` }, { status: 502 });
  }

  const body = await upstream.text();
  const res = new NextResponse(body, { status: upstream.status });
  const uct = upstream.headers.get("content-type");
  if (uct) res.headers.set("content-type", uct);
  return res;
}

type Ctx = { params: Promise<{ path: string[] }> };
const handler = async (req: NextRequest, ctx: Ctx) => forward(req, (await ctx.params).path);

export const GET = handler;
export const POST = handler;
export const PUT = handler;
export const PATCH = handler;
export const DELETE = handler;
