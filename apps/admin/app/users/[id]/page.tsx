import Link from "next/link";
import { adminGql } from "@/lib/gql";
import { getAdminToken } from "@/lib/auth";
import { UserActions } from "./user-actions";

const USER_QUERY = `
  query GetUser($id: ID!) {
    user(id: $id) {
      id username displayName email trustLevel isSuspended createdAt postCount reportCount
    }
  }
`;

const USER_POSTS_QUERY = `
  query UserPosts($authorId: ID!, $limit: Int) {
    posts(authorId: $authorId, limit: $limit) {
      items { id content createdAt replyCount likeCount openReports deletedAt }
    }
  }
`;

interface UserDetail {
  id: string; username: string; displayName: string | null;
  email: string | null; trustLevel: number; isSuspended: boolean;
  createdAt: string; postCount: number; reportCount: number;
}

interface PostItem {
  id: string; content: string; createdAt: string;
  replyCount: number; likeCount: number; openReports: number; deletedAt: string | null;
}

export default async function UserDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const token = await getAdminToken();

  const [userRes, postsRes] = await Promise.allSettled([
    adminGql<{ user: UserDetail | null }>(USER_QUERY, { id }, token ?? undefined),
    adminGql<{ posts: { items: PostItem[] } }>(USER_POSTS_QUERY, { authorId: id, limit: 10 }, token ?? undefined),
  ]);

  const user = userRes.status === "fulfilled" ? userRes.value.user : null;
  const posts = postsRes.status === "fulfilled" ? postsRes.value.posts.items : [];

  if (!user) {
    return (
      <div className="text-center py-16 text-zinc-400">User not found.</div>
    );
  }

  return (
    <div className="max-w-3xl">
      <div className="mb-2 text-sm text-zinc-400">
        <Link href="/users" className="hover:text-zinc-600">Users</Link>
        {" / "}
        <span>@{user.username}</span>
      </div>

      {/* User card */}
      <div className="mb-6 rounded-lg border border-zinc-200 bg-white p-6">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-xl font-semibold text-zinc-900">@{user.username}</h1>
            {user.displayName && <p className="text-sm text-zinc-500">{user.displayName}</p>}
            {user.email && <p className="mt-0.5 text-sm text-zinc-400">{user.email}</p>}
          </div>
          <UserActions userId={user.id} trustLevel={user.trustLevel} isSuspended={user.isSuspended} />
        </div>

        <div className="mt-5 grid grid-cols-4 gap-4 border-t border-zinc-100 pt-4 text-center text-sm">
          <div>
            <p className="text-xs text-zinc-400 uppercase tracking-wide">Trust</p>
            <p className="font-semibold text-zinc-800">L{user.trustLevel}</p>
          </div>
          <div>
            <p className="text-xs text-zinc-400 uppercase tracking-wide">Posts</p>
            <p className="font-semibold text-zinc-800">{user.postCount}</p>
          </div>
          <div>
            <p className="text-xs text-zinc-400 uppercase tracking-wide">Reports</p>
            <p className={`font-semibold ${user.reportCount > 0 ? "text-red-600" : "text-zinc-800"}`}>
              {user.reportCount}
            </p>
          </div>
          <div>
            <p className="text-xs text-zinc-400 uppercase tracking-wide">Status</p>
            <p className={`font-semibold ${user.isSuspended ? "text-red-600" : "text-green-600"}`}>
              {user.isSuspended ? "Suspended" : "Active"}
            </p>
          </div>
        </div>

        <p className="mt-3 text-xs text-zinc-400">
          Joined {new Date(user.createdAt).toLocaleDateString("en", { dateStyle: "medium" })}
        </p>
      </div>

      {/* Recent posts */}
      <h2 className="mb-3 text-sm font-medium text-zinc-500 uppercase tracking-wide">Recent Posts</h2>
      {posts.length === 0 ? (
        <p className="text-sm text-zinc-400">No posts.</p>
      ) : (
        <div className="space-y-2">
          {posts.map((p) => (
            <div
              key={p.id}
              className={`rounded-lg border px-4 py-3 text-sm ${
                p.deletedAt ? "border-zinc-100 bg-zinc-50 opacity-60" : "border-zinc-200 bg-white"
              }`}
            >
              <p className="line-clamp-2 text-zinc-700 whitespace-pre-wrap">{p.content}</p>
              <div className="mt-1.5 flex items-center gap-3 text-xs text-zinc-400">
                <span>{new Date(p.createdAt).toLocaleDateString()}</span>
                <span>{p.replyCount} replies</span>
                <span>{p.likeCount} likes</span>
                {p.openReports > 0 && (
                  <span className="text-red-500 font-medium">{p.openReports} reports</span>
                )}
                {p.deletedAt && <span className="text-zinc-300">[deleted]</span>}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
