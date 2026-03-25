"use client";

// PageArticlesClient — wraps article list with load-more and a "Write article" CTA for members.

import { useState, useEffect } from "react";
import Link from "next/link";
import { useAuth } from "@/lib/auth-context";
import { gqlClient } from "@/lib/gql-client";
import { ArticleCard } from "./article-card";

// ─── GraphQL ──────────────────────────────────────────────────────────────────

const PAGE_ARTICLES_MORE_QUERY = `
  query PageArticlesMore($slug: String!, $after: String, $limit: Int) {
    pageArticles(slug: $slug, after: $after, limit: $limit) {
      items { id title slug publishedAt author { username displayName } }
      nextCursor
      hasMore
    }
  }
`;

const CHECK_MEMBER_QUERY = `
  query CheckPageMemberArticles($pageId: ID!) {
    pageMembers(pageId: $pageId) {
      items { role user { id } }
    }
  }
`;

// ─── Types ────────────────────────────────────────────────────────────────────

interface Article {
  id: string;
  title: string;
  slug: string;
  publishedAt: string | null;
  author: { username: string; displayName: string | null };
}

// ─── Component ────────────────────────────────────────────────────────────────

export function PageArticlesClient({
  pageId,
  pageSlug,
  pageName,
  initialArticles,
  initialNextCursor,
  initialHasMore,
}: {
  pageId: string;
  pageSlug: string;
  pageName: string;
  initialArticles: Article[];
  initialNextCursor: string | null;
  initialHasMore: boolean;
}) {
  const { user, loading: authLoading } = useAuth();
  const [articles, setArticles] = useState<Article[]>(initialArticles);
  const [cursor, setCursor] = useState<string | null>(initialNextCursor);
  const [hasMore, setHasMore] = useState(initialHasMore);
  const [loadingMore, setLoadingMore] = useState(false);
  const [loadError, setLoadError] = useState(false);
  const [isMember, setIsMember] = useState(false);

  useEffect(() => {
    if (!user || authLoading) return;
    gqlClient<{ pageMembers: { items: { role: string; user: { id: string } }[] } }>(
      CHECK_MEMBER_QUERY,
      { pageId }
    )
      .then((data) => {
        const m = data.pageMembers.items.find((e) => e.user.id === user.id);
        setIsMember(!!m && (m.role === "admin" || m.role === "editor"));
      })
      .catch(() => {});
  }, [user, authLoading, pageId]);

  async function loadMore() {
    if (!cursor || loadingMore) return;
    setLoadingMore(true);
    setLoadError(false);
    try {
      const data = await gqlClient<{
        pageArticles: { items: Article[]; nextCursor: string | null; hasMore: boolean };
      }>(PAGE_ARTICLES_MORE_QUERY, { slug: pageSlug, after: cursor, limit: 20 });
      setArticles((prev) => [...prev, ...data.pageArticles.items]);
      setCursor(data.pageArticles.nextCursor);
      setHasMore(data.pageArticles.hasMore);
    } catch {
      setLoadError(true);
    } finally {
      setLoadingMore(false);
    }
  }

  // Enrich articles with page info for ArticleCard
  const enriched = articles.map((a) => ({
    ...a,
    board: { name: pageName, owner: { username: `p/${pageSlug}` } },
  }));

  return (
    <div>
      {/* Write article CTA for members */}
      {isMember && (
        <Link
          href={`/compose/article?pageId=${pageId}&pageSlug=${pageSlug}`}
          className="mb-5 flex items-center gap-3 rounded-xl border border-[var(--app-accent-border)] bg-[var(--app-surface-2)] px-4 py-3 hover:bg-[var(--app-surface-3)] transition-colors"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="shrink-0 text-[var(--app-accent)]">
            <path d="M12 5v14M5 12h14" />
          </svg>
          <span className="text-sm text-[var(--app-text-secondary)]">
            Write a new article for{" "}
            <strong className="text-[var(--app-accent)]">/p/{pageSlug}</strong>
          </span>
        </Link>
      )}

      {enriched.length === 0 ? (
        <div className="rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)] px-6 py-10 text-center text-sm text-[var(--app-text-muted)]">
          No published articles yet.
        </div>
      ) : (
        <>
          <div className="space-y-4">
            {enriched.map((article) => (
              <ArticleCard key={article.id} article={article} />
            ))}
          </div>

          {loadError && (
            <div className="mt-4 text-center">
              <p className="mb-1.5 text-sm text-[var(--app-text-muted)]">Failed to load articles.</p>
              <button
                type="button"
                onClick={() => void loadMore()}
                className="text-sm text-[var(--app-accent)] hover:underline"
              >
                Retry
              </button>
            </div>
          )}

          {hasMore && !loadError && (
            <div className="mt-4 text-center">
              <button
                type="button"
                onClick={() => void loadMore()}
                disabled={loadingMore}
                className="rounded-lg border border-[var(--app-border-inner)] bg-[var(--app-surface-3)] px-5 py-2 text-sm text-[var(--app-text-secondary)] hover:border-[var(--app-accent-border)] hover:text-[var(--app-accent)] transition-colors disabled:opacity-50"
              >
                {loadingMore ? "Loading…" : "Load more articles"}
              </button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
