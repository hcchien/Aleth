// Server-side GraphQL fetch helper.
// Uses GATEWAY_URL directly so Next.js cache tagging works properly.

interface GqlOptions {
  revalidate?: number | false;
  tags?: string[];
  authHeader?: string;
}

interface GqlResponse<T> {
  data?: T;
  errors?: { message: string }[];
}

export async function gql<T>(
  query: string,
  variables?: Record<string, unknown>,
  options: GqlOptions = {}
): Promise<T> {
  const url =
    (process.env.GATEWAY_URL || "http://localhost:4000") + "/graphql";

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  if (options.authHeader) {
    headers["Authorization"] = options.authHeader;
  }

  const nextOptions: { revalidate?: number | false; tags?: string[] } = {};
  if (options.revalidate !== undefined) nextOptions.revalidate = options.revalidate;
  if (options.tags) nextOptions.tags = options.tags;

  const res = await fetch(url, {
    method: "POST",
    headers,
    body: JSON.stringify({ query, variables }),
    next: nextOptions,
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
