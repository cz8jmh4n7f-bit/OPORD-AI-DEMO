import { cookies } from "next/headers";
import { NextResponse } from "next/server";

// Clears the HttpOnly key cookie (client JS can't delete it) + the readable user.
export async function POST(): Promise<NextResponse> {
  const jar = await cookies();
  jar.delete("opord_key");
  jar.delete("opord_user");
  return NextResponse.json({ ok: true });
}
