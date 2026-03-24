import { notFound } from "next/navigation";
import { getTranslations } from "next-intl/server";
import Link from "next/link";
import { gql } from "@/lib/gql";
import { ForumShell } from "../../components/forum-shell";
import { PageFollowButton } from "../../components/page-follow-button";
import { PageFeedClient } from "../../components/page-feed-client";
import { PageArticlesClient } from "../../components/page-articles-client";

const PAGE_QUERY = `
  query GetFanPage($slug: String!) {
    page(slug: $slug) {
      id
      slug
      name
      description
      avatarUrl
      coverUrl
      category
      apEnabled
      followerCount
      isFollowing
    }
  }
`;

const PAGE_ARTICLES_QUERY = `
  query PageArticles($slug: String!, $limit: Int) {
    pageArticles(slug: $slug, limit: $limit) {
      items { id title slug publishedAt author { username displayName } }
      nextCursor
      hasMore
    }
  }
`;

const PAGE_FEED_QUERY = `
  query PageFeed($slug: String!, $limit: Int) {
    pageFeed(slug: $slug, limit: $limit) {
      items {
        id
        content
        createdAt
        author { id username displayName }
      }
      nextCursor
      hasMore
    }
  }
`;

interface FanPageData {
  id: string;
  slug: string;
  name: string;
  description: string | null;
  avatarUrl: string | null;
  coverUrl: string | null;
  category: string;
  apEnabled: boolean;
  followerCount: number;
  isFollowing: boolean;
}

interface Post {
  id: string;
  content: string;
  createdAt: string;
  author: { id: string; username: string; displayName: string | null };
}

interface Article {
  id: string;
  title: string;
  slug: string;
  publishedAt: string | null;
  author: { username: string; displayName: string | null };
}

interface ArticleConnection {
  items: Article[];
  nextCursor: string | null;
  hasMore: boolean;
}

interface PostConnection {
  items: Post[];
  nextCursor: string | null;
  hasMore: boolean;
}

interface PageProps {
  params: Promise<{ slug: string }>;
  searchParams: Promise<{ tab?: string }>;
}

export default async function FanPageView({ params, searchParams }: PageProps) {
  const { slug } = await params;
  const { tab = "posts" } = await searchParams;
  const t = await getTranslations("fanPage");

  let page: FanPageData | null = null;
  try {
    const data = await gql<{ page: FanPageData | null }>(
      PAGE_QUERY,
      { slug },
      { revalidate: 60 }
    );
    page = data.page;
  } catch {
    // gateway unavailable
  }

  if (!page) {
    notFound();
  }

  let articles: ArticleConnection = { items: [], nextCursor: null, hasMore: false };
  let posts: PostConnection = { items: [], nextCursor: null, hasMore: false };

  const [articlesResult, postsResult] = await Promise.allSettled([
    gql<{ pageArticles: ArticleConnection }>(
      PAGE_ARTICLES_QUERY,
      { slug, limit: 20 },
      { revalidate: 60 }
    ),
    gql<{ pageFeed: PostConnection }>(
      PAGE_FEED_QUERY,
      { slug, limit: 20 },
      { revalidate: 30 }
    ),
  ]);
  if (articlesResult.status === "fulfilled") articles = articlesResult.value.pageArticles;
  if (postsResult.status === "fulfilled") posts = postsResult.value.pageFeed;

  const avatarChar = (page.name[0] || "P").toUpperCase();

  return (
    <ForumShell>
      {/* Cover + Header */}
      <div className="mb-8 overflow-hidden rounded-2xl border border-[var(--app-border-2)] bg-[var(--app-surface)]">
        {/* Cover image */}
        {page.coverUrl ? (
          <div className="h-40 w-full overflow-hidden">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img src={page.coverUrl} alt="" className="h-full w-full object-cover" />
          </div>
        ) : (
          <div className="h-40 w-full bg-gradient-to-br from-[#1a1f2e] to-[#0c0f17]" />
        )}

        <div className="px-6 pb-6">
          {/* Avatar + action row */}
          <div className="-mt-10 mb-4 flex items-end justify-between">
            <div className="flex h-20 w-20 items-center justify-center rounded-full border-4 border-[var(--app-bg)] bg-[var(--app-accent-bg)] text-3xl font-semibold text-[var(--app-accent)] overflow-hidden shrink-0">
              {page.avatarUrl ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={page.avatarUrl} alt={page.name} className="h-full w-full object-cover" />
              ) : (
                avatarChar
              )}
            </div>

            <div className="flex items-center gap-2 mt-4">
              <PageFollowButton
                pageId={page.id}
                slug={page.slug}
                initialFollowerCount={page.followerCount}
                initialIsFollowing={page.isFollowing}
              />
              <Link
                href={`/p/${page.slug}/admin`}
                className="rounded-full border border-[var(--app-border)] px-4 py-1.5 text-sm text-[var(--app-text-secondary)] hover:border-[var(--app-border-hover)] transition-colors"
              >
                {t("adminPanel")}
              </Link>
            </div>
          </div>

          {/* Page info */}
          <h1 className="font-serif text-2xl text-[var(--app-text-heading)]">{page.name}</h1>
          <div className="mt-1 flex items-center gap-2 text-sm text-[var(--app-text-muted)]">
            <span>/p/{page.slug}</span>
            <span>·</span>
            <span className="capitalize">{page.category}</span>
            {page.apEnabled && (
              <>
                <span>·</span>
                <span className="text-emerald-400">Fediverse</span>
              </>
            )}
          </div>
          {page.description && (
            <p className="mt-3 max-w-lg text-sm leading-relaxed text-[var(--app-text-bright)]">
              {page.description}
            </p>
          )}
          <div className="mt-3 text-sm text-[var(--app-text-secondary)]">
            <span className="font-medium text-[var(--app-text-bright)]">{page.followerCount}</span>{" "}
            {t("followers")}
          </div>
        </div>
      </div>

      {/* Tab navigation */}
      <div className="mb-6 flex gap-1 border-b border-[var(--app-border)]">
        <Link
          href={`/p/${page.slug}?tab=posts`}
          className={`px-4 py-2 text-sm transition-colors ${
            tab === "posts"
              ? "border-b-2 border-[var(--app-accent)] text-[var(--app-text-heading)]"
              : "text-[var(--app-text-secondary)] hover:text-[var(--app-text)]"
          }`}
        >
          {t("posts")}
          <span className="ml-1.5 font-mono text-xs text-[var(--app-text-muted)]">
            ({posts.items.length}{posts.hasMore ? "+" : ""})
          </span>
        </Link>
        <Link
          href={`/p/${page.slug}?tab=articles`}
          className={`px-4 py-2 text-sm transition-colors ${
            tab === "articles"
              ? "border-b-2 border-[var(--app-accent)] text-[var(--app-text-heading)]"
              : "text-[var(--app-text-secondary)] hover:text-[var(--app-text)]"
          }`}
        >
          {t("articles")}
          <span className="ml-1.5 font-mono text-xs text-[var(--app-text-muted)]">
            ({articles.items.length}{articles.hasMore ? "+" : ""})
          </span>
        </Link>
      </div>

      {/* Tab content */}
      {tab === "posts" && (
        <PageFeedClient
          pageId={page.id}
          pageSlug={page.slug}
          initialPosts={posts.items}
          initialNextCursor={posts.nextCursor}
          initialHasMore={posts.hasMore}
        />
      )}

      {tab === "articles" && (
        <PageArticlesClient
          pageId={page.id}
          pageSlug={page.slug}
          pageName={page.name}
          initialArticles={articles.items}
          initialNextCursor={articles.nextCursor}
          initialHasMore={articles.hasMore}
        />
      )}
    </ForumShell>
  );
}
