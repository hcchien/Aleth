// Typed GraphQL client for the admin service.
// Server-side calls go directly to ADMIN_API_URL.
// Client-side calls go through the /api/admin/graphql rewrite.

const ADMIN_API =
  typeof window === "undefined"
    ? `${process.env.ADMIN_API_URL ?? "http://localhost:8087"}/graphql`
    : "/api/admin/graphql";

export async function adminGql<T>(
  query: string,
  variables?: Record<string, unknown>,
  token?: string
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  if (token) headers["Authorization"] = `Bearer ${token}`;

  const res = await fetch(ADMIN_API, {
    method: "POST",
    headers,
    body: JSON.stringify({ query, variables }),
    cache: "no-store",
  });

  const json = (await res.json()) as { data?: T; errors?: { message: string }[] };
  if (json.errors?.length) throw new Error(json.errors[0].message);
  if (!json.data) throw new Error("No data returned");
  return json.data;
}
