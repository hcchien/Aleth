import { cookies } from "next/headers";

const COOKIE = "admin_token";

export async function getAdminToken(): Promise<string | null> {
  const store = await cookies();
  return store.get(COOKIE)?.value ?? null;
}

export async function setAdminToken(token: string): Promise<void> {
  const store = await cookies();
  store.set(COOKIE, token, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: 8 * 60 * 60, // 8 hours — matches JWT TTL
  });
}

export async function clearAdminToken(): Promise<void> {
  const store = await cookies();
  store.delete(COOKIE);
}
