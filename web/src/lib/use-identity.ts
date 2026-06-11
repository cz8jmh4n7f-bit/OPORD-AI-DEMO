"use client";

import { useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { getStoredUser, logoutRequest } from "./client-auth";

const API = "/bff";

export type Me = { email: string; tenant: string; role: string };

// useIdentity resolves the signed-in identity from GET /api/v1/me via the /bff
// proxy (the HttpOnly key cookie rides along automatically; dev mode returns the
// injected admin, auth mode reflects the cookie or 401s). hasKey reflects the
// readable `opord_user` cookie, set only on a real sign-in. Shared by the desktop
// sidebar + mobile drawer; re-runs on navigation.
export function useIdentity() {
  const pathname = usePathname();
  const router = useRouter();
  const [me, setMe] = useState<Me | null>(null);
  const [hasKey, setHasKey] = useState(false);

  useEffect(() => {
    let cancelled = false;
    fetch(`${API}/api/v1/me`)
      .then((res) => (res.ok ? (res.json() as Promise<Me>) : null))
      .then((data) => {
        if (cancelled) return;
        setMe(data);
        setHasKey(!!getStoredUser());
      })
      .catch(() => {
        if (cancelled) return;
        setMe(null);
        setHasKey(false);
      });
    return () => {
      cancelled = true;
    };
  }, [pathname]);

  async function logout() {
    await logoutRequest();
    setMe(null);
    setHasKey(false);
    router.push("/login");
    router.refresh();
  }

  return { me, hasKey, logout };
}
