import { NextResponse } from "next/server";
import { clearAdminToken } from "@/lib/auth";

export async function POST() {
  await clearAdminToken();
  return NextResponse.redirect(new URL("/login", process.env.NEXT_PUBLIC_BASE_URL ?? "http://localhost:3001"));
}
