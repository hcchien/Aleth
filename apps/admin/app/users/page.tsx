import Link from "next/link";
import { adminGql } from "@/lib/gql";
import { getAdminToken } from "@/lib/auth";

const USERS_QUERY = `
  query Users($search: String, $trustLevel: Int, $suspended: Boolean, $after: String, $limit: Int) {
    users(search: $search, trustLevel: $trustLevel, suspended: $suspended, after: $after, limit: $limit) {
      hasMore nextCursor
      items {
        id username displayName email trustLevel isSuspended createdAt postCount reportCount
      }
    }
  }
`;

interface UserSummary {
  id: string; username: string; displayName: string | null;
  email: string | null; trustLevel: number; isSuspended: boolean;
  createdAt: string; postCount: number; reportCount: number;
}

interface UsersData {
  users: { hasMore: boolean; nextCursor: string | null; items: UserSummary[] };
}

export default async function UsersPage({
  searchParams,
}: {
  searchParams: Promise<{ search?: string; trust?: string; suspended?: string; after?: string }>;
}) {
  const { search, trust, suspended, after } = await searchParams;
  const token = await getAdminToken();
  let data: UsersData | null = null;
  try {
    data = await adminGql<UsersData>(USERS_QUERY, {
      search: search || undefined,
      trustLevel: trust ? parseInt(trust) : undefined,
      suspended: suspended === "true" ? true : suspended === "false" ? false : undefined,
      after,
      limit: 50,
    }, token ?? undefined);
  } catch { /* show empty */ }

  const users = data?.users.items ?? [];

  return (
    <div>
      <h1 className="mb-6 text-2xl font-semibold">Users</h1>

      {/* Filters */}
      <form method="get" className="mb-5 flex flex-wrap gap-3">
        <input
          name="search"
          defaultValue={search}
          placeholder="Search username or email…"
          className="rounded-md border border-zinc-300 px-3 py-1.5 text-sm outline-none focus:border-zinc-500 w-60"
        />
        <select name="trust" defaultValue={trust ?? ""} className="rounded-md border border-zinc-300 px-3 py-1.5 text-sm">
          <option value="">All trust levels</option>
          {[0,1,2,3,4,5].map(l => <option key={l} value={l}>Trust {l}</option>)}
        </select>
        <select name="suspended" defaultValue={suspended ?? ""} className="rounded-md border border-zinc-300 px-3 py-1.5 text-sm">
          <option value="">All users</option>
          <option value="false">Active</option>
          <option value="true">Suspended</option>
        </select>
        <button type="submit" className="rounded-md bg-zinc-900 px-4 py-1.5 text-sm text-white hover:bg-zinc-700">
          Filter
        </button>
        {(search || trust || suspended) && (
          <a href="/users" className="rounded-md border border-zinc-200 px-4 py-1.5 text-sm text-zinc-500 hover:bg-zinc-50">
            Clear
          </a>
        )}
      </form>

      {/* Table */}
      <div className="rounded-lg border border-zinc-200 bg-white overflow-hidden">
        <table className="w-full text-sm">
          <thead className="border-b border-zinc-100 bg-zinc-50 text-xs font-medium uppercase tracking-wider text-zinc-400">
            <tr>
              <th className="px-4 py-3 text-left">User</th>
              <th className="px-4 py-3 text-left">Email</th>
              <th className="px-4 py-3 text-center">Trust</th>
              <th className="px-4 py-3 text-center">Posts</th>
              <th className="px-4 py-3 text-center">Reports</th>
              <th className="px-4 py-3 text-left">Joined</th>
              <th className="px-4 py-3 text-left">Status</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-zinc-100">
            {users.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-4 py-8 text-center text-zinc-400">No users found.</td>
              </tr>
            ) : users.map((u) => (
              <tr key={u.id} className="hover:bg-zinc-50 transition-colors">
                <td className="px-4 py-3">
                  <Link href={`/users/${u.id}`} className="font-medium text-zinc-800 hover:text-zinc-600">
                    @{u.username}
                  </Link>
                  {u.displayName && <span className="ml-1.5 text-xs text-zinc-400">{u.displayName}</span>}
                </td>
                <td className="px-4 py-3 text-zinc-500">{u.email ?? "—"}</td>
                <td className="px-4 py-3 text-center">
                  <span className="inline-block rounded bg-zinc-100 px-2 py-0.5 text-xs font-medium text-zinc-600">
                    L{u.trustLevel}
                  </span>
                </td>
                <td className="px-4 py-3 text-center text-zinc-500">{u.postCount}</td>
                <td className="px-4 py-3 text-center">
                  {u.reportCount > 0 ? (
                    <span className="text-red-500 font-medium">{u.reportCount}</span>
                  ) : (
                    <span className="text-zinc-400">0</span>
                  )}
                </td>
                <td className="px-4 py-3 text-zinc-400 text-xs">
                  {new Date(u.createdAt).toLocaleDateString()}
                </td>
                <td className="px-4 py-3">
                  {u.isSuspended ? (
                    <span className="rounded bg-red-50 px-2 py-0.5 text-xs font-medium text-red-600">Suspended</span>
                  ) : (
                    <span className="rounded bg-green-50 px-2 py-0.5 text-xs font-medium text-green-600">Active</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {data?.users.hasMore && (
        <div className="mt-4 text-center">
          <a
            href={`/users?${new URLSearchParams({ ...(search && { search }), ...(trust && { trust }), ...(suspended && { suspended }), after: data.users.nextCursor! }).toString()}`}
            className="rounded-md border border-zinc-200 bg-white px-4 py-2 text-sm text-zinc-600 hover:bg-zinc-50"
          >
            Load more
          </a>
        </div>
      )}
    </div>
  );
}
