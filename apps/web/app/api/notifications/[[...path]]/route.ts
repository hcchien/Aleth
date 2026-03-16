import { type NextRequest, NextResponse } from "next/server";

const NOTIFICATION_URL =
  process.env.NOTIFICATION_SERVICE_URL ?? "http://localhost:8084";

async function handler(
  req: NextRequest,
  { params }: { params: Promise<{ path?: string[] }> },
) {
  const { path = [] } = await params;
  const pathStr = path.length > 0 ? `/${path.join("/")}` : "";
  const upstream = new URL(`${NOTIFICATION_URL}/notifications${pathStr}`);

  req.nextUrl.searchParams.forEach((v, k) => upstream.searchParams.set(k, v));

  const headers: Record<string, string> = {};
  const auth = req.headers.get("authorization");
  if (auth) headers["authorization"] = auth;
  const ct = req.headers.get("content-type");
  if (ct) headers["content-type"] = ct;

  const body =
    req.method === "POST" || req.method === "PUT" || req.method === "PATCH"
      ? await req.text()
      : undefined;

  try {
    const res = await fetch(upstream.toString(), {
      method: req.method,
      headers,
      body,
    });
    const text = await res.text();
    return new NextResponse(text, {
      status: res.status,
      headers: {
        "content-type":
          res.headers.get("content-type") ?? "application/json",
      },
    });
  } catch {
    return NextResponse.json(
      { error: "notification service unavailable" },
      { status: 503 },
    );
  }
}

export const GET = handler;
export const POST = handler;
