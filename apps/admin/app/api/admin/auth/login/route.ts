import { NextRequest, NextResponse } from "next/server";
import { adminGql } from "@/lib/gql";
import { setAdminToken } from "@/lib/auth";

const LOGIN_MUTATION = `
  mutation AdminLogin($username: String!, $password: String!) {
    adminLogin(username: $username, password: $password) {
      token
      admin { id username role }
    }
  }
`;

interface LoginResult {
  adminLogin: { token: string; admin: { id: string; username: string; role: string } };
}

export async function POST(req: NextRequest) {
  try {
    const { username, password } = (await req.json()) as { username: string; password: string };
    const data = await adminGql<LoginResult>(LOGIN_MUTATION, { username, password });
    await setAdminToken(data.adminLogin.token);
    return NextResponse.json({ ok: true });
  } catch (err) {
    return NextResponse.json(
      { error: err instanceof Error ? err.message : "Login failed" },
      { status: 401 }
    );
  }
}
