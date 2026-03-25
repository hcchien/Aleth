import { adminGql } from "@/lib/gql";
import { getAdminToken } from "@/lib/auth";
import { DeletePostButton } from "./delete-post-button";

const POSTS_QUERY = `
  query Posts($after: String, $limit: Int) {
    posts(after: $after, limit: $limit) {
      hasMore nextCursor
      items {
        id content authorId authorUsername replyCount likeCount openReports createdAt deletedAt
      }
    }
  }
`;

interface PostItem {
  id: string; content: string; authorId: string; authorUsername: string;
  replyCount: number; likeCount: number; openReports: number;
  createdAt: string; deletedAt: string | null;
}

interface PostsData { posts: { hasMore: boolean; nextCursor: string | null; items: PostItem[] } }

export default async function PostsPage({
  searchParams,
}: {
  searchParams: Promise<{ after?: string }>;
}) {
  const { after } = await searchParams;
  const token = await getAdminToken();
  let data: PostsData | null = null;
  try {
    data = await adminGql<PostsData>(POSTS_QUERY, { after, limit: 30 }, token ?? undefined);
  } catch { /* show empty */ }

  const posts = data?.posts.items ?? [];

  return (
    <div>
      <h1 className="mb-6 text-2xl font-semibold">Posts</h1>
      <div className="rounded-lg border border-zinc-200 bg-white overflow-hidden">
        <table className="w-full text-sm">
          <thead className="border-b border-zinc-100 bg-zinc-50 text-xs font-medium uppercase tracking-wider text-zinc-400">
            <tr>
              <th className="px-4 py-3 text-left">Content</th>
              <th className="px-4 py-3 text-left">Author</th>
              <th className="px-4 py-3 text-center">Replies</th>
              <th className="px-4 py-3 text-center">Likes</th>
              <th className="px-4 py-3 text-center">Reports</th>
              <th className="px-4 py-3 text-left">Date</th>
              <th className="px-4 py-3" />
            </tr>
          </thead>
          <tbody className="divide-y divide-zinc-100">
            {posts.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-4 py-8 text-center text-zinc-400">No posts.</td>
              </tr>
            ) : posts.map((p) => (
              <tr key={p.id} className={`hover:bg-zinc-50 transition-colors ${p.deletedAt ? "opacity-50" : ""}`}>
                <td className="px-4 py-3 max-w-xs">
                  <p className="line-clamp-2 text-zinc-700 whitespace-pre-wrap">{p.content}</p>
                  {p.deletedAt && <span className="text-xs text-zinc-400">[deleted]</span>}
                </td>
                <td className="px-4 py-3 text-zinc-500">@{p.authorUsername}</td>
                <td className="px-4 py-3 text-center text-zinc-400">{p.replyCount}</td>
                <td className="px-4 py-3 text-center text-zinc-400">{p.likeCount}</td>
                <td className="px-4 py-3 text-center">
                  {p.openReports > 0 ? (
                    <span className="font-medium text-red-500">{p.openReports}</span>
                  ) : (
                    <span className="text-zinc-300">—</span>
                  )}
                </td>
                <td className="px-4 py-3 text-xs text-zinc-400">
                  {new Date(p.createdAt).toLocaleDateString()}
                </td>
                <td className="px-4 py-3">
                  {!p.deletedAt && <DeletePostButton postId={p.id} />}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {data?.posts.hasMore && (
        <div className="mt-4 text-center">
          <a
            href={`/posts?after=${data.posts.nextCursor}`}
            className="rounded-md border border-zinc-200 bg-white px-4 py-2 text-sm text-zinc-600 hover:bg-zinc-50"
          >
            Load more
          </a>
        </div>
      )}
    </div>
  );
}
