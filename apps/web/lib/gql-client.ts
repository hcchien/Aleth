"use client";

// Client-side GraphQL fetch helper.
// Uses /graphql (proxied by Next.js to the gateway) and reads the auth token from localStorage.

import { getAccessToken } from "./auth";

interface GqlResponse<T> {
  data?: T;
  errors?: { message: string }[];
}

export async function gqlClient<T>(
  query: string,
  variables?: Record<string, unknown>
): Promise<T> {
  const url = process.env.NEXT_PUBLIC_API_URL || "/graphql";

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };

  const token = getAccessToken();
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(url, {
    method: "POST",
    headers,
    body: JSON.stringify({ query, variables }),
  });

  if (!res.ok) {
    throw new Error(`GraphQL request failed: ${res.status}`);
  }

  const json: GqlResponse<T> = await res.json();
  if (json.errors?.length) {
    throw new Error(json.errors[0].message);
  }
  if (!json.data) {
    throw new Error("No data in GraphQL response");
  }
  return json.data;
}
