"use client";

// PageFeedClient — wraps the fan-page post list with compose box + load-more.
// Receives SSR-fetched initial posts from the server component.

import { useState, useCallback } from "react";
import Link from "next/link";
import { gqlClient } from "@/lib/gql-client";
import { PageComposeBox, type PagePost } from "./page-compose-box";

// ─── GraphQL ──────────────────────────────────────────────────────────────────

const PAGE_FEED_MORE_QUERY = `
  query PageFeedMore($slug: String!, $after: String, $limit: Int) {
    pageFeed(slug: $slug, after: $after, limit: $limit) {
      items {
        id content createdAt
        author { id username displayName }
      }
      nextCursor
      hasMore
    }
  }
`;

// ─── Helpers ──────────────────────────────────────────────────────────────────

function timeAgo(isoString: string): string {
  const diff = Date.now() - new Date(isoString).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d`;
  return new Date(isoString).toLocaleDateString();
}

function PostRow({ post }: { post: PagePost }) {
  return (
    <article className="px-5 py-4">
      <div className="mb-2 flex items-center gap-2 text-xs text-[var(--app-text-muted)]">
        <Link
          href={`/@${post.author.username}`}
          className="font-medium text-[var(--app-text-secondary)] hover:text-[var(--app-text)]"
        >
          {post.author.displayName ?? post.author.username}
        </Link>
        <span>·</span>
        <span>{timeAgo(post.createdAt)}</span>
      </div>
      <p className="text-sm text-[var(--app-text-bright)] whitespace-pre-wrap break-words">
        {post.content}
      </p>
    </article>
  );
}

// ─── Component ────────────────────────────────────────────────────────────────

export function PageFeedClient({
  pageId,
  pageSlug,
  initialPosts,
  initialNextCursor,
  initialHasMore,
}: {
  pageId: string;
  pageSlug: string;
  initialPosts: PagePost[];
  initialNextCursor: string | null;
  initialHasMore: boolean;
}) {
  const [posts, setPosts] = useState<PagePost[]>(initialPosts);
  const [cursor, setCursor] = useState<string | null>(initialNextCursor);
  const [hasMore, setHasMore] = useState(initialHasMore);
  const [loadingMore, setLoadingMore] = useState(false);

  const handlePosted = useCallback((post: PagePost) => {
    setPosts((prev) => [post, ...prev]);
  }, []);

  async function loadMore() {
    if (!cursor || loadingMore) return;
    setLoadingMore(true);
    try {
      const data = await gqlClient<{
        pageFeed: { items: PagePost[]; nextCursor: string | null; hasMore: boolean };
      }>(PAGE_FEED_MORE_QUERY, { slug: pageSlug, after: cursor, limit: 20 });
      setPosts((prev) => [...prev, ...data.pageFeed.items]);
      setCursor(data.pageFeed.nextCursor);
      setHasMore(data.pageFeed.hasMore);
    } catch {
      // fail silently — user can retry
    } finally {
      setLoadingMore(false);
    }
  }

  return (
    <div>
      <PageComposeBox pageId={pageId} pageSlug={pageSlug} onPosted={handlePosted} />

      {posts.length === 0 ? (
        <div className="rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)] px-6 py-10 text-center text-sm text-[var(--app-text-muted)]">
          No posts yet.
        </div>
      ) : (
        <>
          <div className="divide-y divide-[var(--app-border)] rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)]">
            {posts.map((post) => (
              <PostRow key={post.id} post={post} />
            ))}
          </div>

          {hasMore && (
            <div className="mt-4 text-center">
              <button
                type="button"
                onClick={() => void loadMore()}
                disabled={loadingMore}
                className="rounded-lg border border-[var(--app-border-inner)] bg-[var(--app-surface-3)] px-5 py-2 text-sm text-[var(--app-text-secondary)] hover:border-[var(--app-accent-border)] hover:text-[var(--app-accent)] transition-colors disabled:opacity-50"
              >
                {loadingMore ? "Loading…" : "Load more posts"}
              </button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
