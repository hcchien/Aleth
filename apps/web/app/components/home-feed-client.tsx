"use client";

import { useState, useEffect, useRef } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useAuth } from "@/lib/auth-context";
import { gqlClient } from "@/lib/gql-client";
import { ReactBar } from "./react-bar";
import type { SignatureInfo } from "./signature-badge";

// ─── Types ───────────────────────────────────────────────────────────────────

interface UserSummary {
  id: string;
  username: string;
  displayName: string | null;
  trustLevel: number;
}

interface ResharedPost {
  id: string;
  content: string;
  createdAt: string;
  author: UserSummary;
}

export interface PostItem {
  id: string;
  content: string;
  replyCount: number;
  likeCount: number;
  viewerEmotion?: string | null;
  reactionCounts: { emotion: string; count: number }[];
  createdAt: string;
  resharedFromId?: string | null;
  resharedFrom?: ResharedPost | null;
  signatureInfo: SignatureInfo;
  author: UserSummary;
}

export interface ArticleItem {
  id: string;
  slug: string;
  title: string;
  contentMd: string | null;
  publishedAt: string | null;
  signatureInfo: SignatureInfo;
  author: UserSummary;
  board: {
    id: string;
    name: string;
    subscriberCount: number;
    owner: { username: string };
  };
}

export interface FeedItem {
  id: string;
  type: string;
  post: PostItem | null;
  article: ArticleItem | null;
}

interface FeedConnection {
  items: FeedItem[];
  nextCursor: string | null;
  hasMore: boolean;
}

// ─── GraphQL queries ──────────────────────────────────────────────────────────

const POST_FIELDS = `
  id content replyCount likeCount viewerEmotion
  reactionCounts { emotion count }
  createdAt resharedFromId
  resharedFrom { id content createdAt author { id username displayName trustLevel } }
  signatureInfo { isSigned isVerified explanation }
  author { id username displayName trustLevel }
`;

const ARTICLE_FIELDS = `
  id slug title contentMd publishedAt
  signatureInfo { isSigned isVerified explanation }
  author { id username displayName trustLevel }
  board { id name subscriberCount owner { username } }
`;

const PERSONALIZED_FEED_QUERY = `
  query Feed($after: String, $limit: Int) {
    feed(after: $after, limit: $limit) {
      items { id type post { ${POST_FIELDS} } article { ${ARTICLE_FIELDS} } }
      nextCursor hasMore
    }
  }
`;

const EXPLORE_FEED_MORE_QUERY = `
  query ExploreFeedMore($after: String, $limit: Int) {
    exploreFeed(after: $after, limit: $limit) {
      items { id type post { ${POST_FIELDS} } article { ${ARTICLE_FIELDS} } }
      nextCursor hasMore
    }
  }
`;

// ─── Helpers ─────────────────────────────────────────────────────────────────

const AVATAR_COLORS = [
  "bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300",
  "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300",
  "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300",
  "bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300",
  "bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-300",
  "bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-300",
];

function avatarColor(username: string): string {
  let hash = 0;
  for (let i = 0; i < username.length; i++) hash = username.charCodeAt(i) + ((hash << 5) - hash);
  return AVATAR_COLORS[Math.abs(hash) % AVATAR_COLORS.length];
}

function LevelBadge({ level }: { level: number }) {
  const icon = level >= 4 ? "♛" : "⬡";
  const cls =
    level >= 4
      ? "border-amber-400/40 bg-amber-50 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300"
      : level >= 3
      ? "border-orange-400/40 bg-orange-50 text-orange-700 dark:bg-orange-500/15 dark:text-orange-300"
      : level >= 2
      ? "border-lime-400/40 bg-lime-50 text-lime-700 dark:bg-lime-500/15 dark:text-lime-300"
      : "border-sky-400/40 bg-sky-50 text-sky-700 dark:bg-sky-500/15 dark:text-sky-300";
  return (
    <span className={`inline-flex items-center gap-0.5 rounded-full border px-2 py-0.5 text-xs font-medium ${cls}`}>
      <span className="text-[0.6rem] leading-none">{icon}</span>
      <span>L{level}</span>
    </span>
  );
}

function formatDate(iso: string | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleDateString(undefined, { month: "numeric", day: "numeric" });
}

function textPreview(text: string, maxLen = 140): string {
  const normalized = text.replace(/\s+/g, " ").trim();
  if (normalized.length <= maxLen) return normalized;
  return `${normalized.slice(0, maxLen)}…`;
}

// ─── Main component ───────────────────────────────────────────────────────────

interface HomeFeedClientProps {
  initialItems: FeedItem[];
  initialCursor: string | null;
  initialHasMore: boolean;
}

export function HomeFeedClient({ initialItems, initialCursor, initialHasMore }: HomeFeedClientProps) {
  const t = useTranslations("feed");
  const tCommon = useTranslations("common");
  const { user, loading: authLoading } = useAuth();
  const [items, setItems] = useState<FeedItem[]>(initialItems);
  const [cursor, setCursor] = useState<string | null>(initialCursor);
  const [hasMore, setHasMore] = useState(initialHasMore);
  const [feedType, setFeedType] = useState<"explore" | "personalized">("explore");
  const [loading, setLoading] = useState(false);
  const fetchedForUserRef = useRef<string | undefined>(undefined);

  // Once auth resolves, fetch personalized feed if logged in.
  // Re-runs if the user logs in after the page was already loaded as a guest.
  useEffect(() => {
    if (authLoading) return;
    if (!user) return; // guest: keep SSR explore feed
    if (fetchedForUserRef.current === user.id) return; // already fetched for this user
    fetchedForUserRef.current = user.id;

    gqlClient<{ feed: FeedConnection }>(PERSONALIZED_FEED_QUERY, { limit: 20 })
      .then((data) => {
        // Only switch to personalized feed if the service returned content.
        // An empty result with hasMore=false means the feed service is down or
        // unreachable (gateway degrades silently); keep the SSR explore items.
        if (data.feed.items.length > 0 || data.feed.hasMore) {
          setItems(data.feed.items);
          setCursor(data.feed.nextCursor);
          setHasMore(data.feed.hasMore);
          setFeedType("personalized");
        }
      })
      .catch(() => {
        // fallback: keep explore feed
      });
  }, [authLoading, user]);

  async function loadMore() {
    if (loading || !hasMore) return;
    setLoading(true);
    try {
      if (feedType === "personalized") {
        const data = await gqlClient<{ feed: FeedConnection }>(
          PERSONALIZED_FEED_QUERY,
          { after: cursor, limit: 20 }
        );
        setItems((prev) => [...prev, ...data.feed.items]);
        setCursor(data.feed.nextCursor);
        setHasMore(data.feed.hasMore);
      } else {
        const data = await gqlClient<{ exploreFeed: FeedConnection }>(
          EXPLORE_FEED_MORE_QUERY,
          { after: cursor, limit: 20 }
        );
        setItems((prev) => [...prev, ...data.exploreFeed.items]);
        setCursor(data.exploreFeed.nextCursor);
        setHasMore(data.exploreFeed.hasMore);
      }
    } catch (err) {
      console.error("Failed to load more:", err);
    } finally {
      setLoading(false);
    }
  }

  const headingText = feedType === "personalized" ? t("myFeed") : t("allFeed");
  const composeText =
    user
      ? user.trustLevel >= 1
        ? t("composeNewPost")
        : t("composeUpgrade")
      : t("composeSignIn");

  return (
    <>
      <h1
        className="mb-6 text-3xl font-bold text-[var(--app-text-heading)]"
        style={{ fontFamily: "var(--font-playfair), 'Playfair Display', Georgia, serif" }}
      >
        {headingText}
      </h1>

      <Link
        href="/compose"
        className="mb-8 block w-full rounded-md border border-[var(--app-border)] bg-[var(--app-surface)] px-4 py-3 text-center text-sm text-[var(--app-text-muted)] hover:border-[var(--app-accent-border)] hover:text-[var(--app-accent)] transition-colors"
      >
        {composeText}
      </Link>

      {items.length === 0 && !authLoading ? (
        <div className="rounded-xl border border-[var(--app-border)] bg-[var(--app-surface)] px-6 py-10 text-center text-sm text-[var(--app-text-secondary)]">
          {feedType === "personalized" ? t("emptyPersonalized") : t("emptyExplore")}
        </div>
      ) : (
        <div className="divide-y divide-[var(--app-border)]">
          {items.map((item) => (
            <FeedCard key={item.id} item={item} />
          ))}
        </div>
      )}

      {hasMore && (
        <div className="py-6 text-center">
          <button
            onClick={loadMore}
            disabled={loading}
            className="text-sm text-[var(--app-text-muted)] hover:text-[var(--app-text-secondary)] disabled:opacity-50 transition-colors"
          >
            {loading ? tCommon("loading") : t("loadMore")}
          </button>
        </div>
      )}
    </>
  );
}

// ─── SignatureMark ────────────────────────────────────────────────────────────
// Shows a trust-level-colored checkmark when content is signed+verified,
// or a neutral warning when signed but not verified.

function signatureColor(trustLevel: number): string {
  if (trustLevel >= 3) return "text-amber-400";   // gold
  if (trustLevel === 2) return "text-lime-400";   // green
  return "text-sky-400";                           // blue (L0, L1)
}

function SignatureMark({
  isVerified,
  trustLevel,
  explanation,
}: {
  isVerified: boolean;
  trustLevel: number;
  explanation: string;
}) {
  if (!isVerified) {
    return (
      <span className="text-xs text-amber-500/70" title={explanation}>
        ⚠
      </span>
    );
  }
  return (
    <span
      className={`text-xs font-semibold ${signatureColor(trustLevel)}`}
      title={explanation}
    >
      ✓
    </span>
  );
}

// ─── FeedCard ─────────────────────────────────────────────────────────────────

function FeedCard({ item }: { item: FeedItem }) {
  const t = useTranslations("feed");
  const author = item.post?.author ?? item.article?.author;
  if (!author) return null;

  const name = author.displayName ?? author.username;
  const trust = Math.max(1, author.trustLevel);
  const isReshare = !!(item.post?.resharedFrom);
  const resharedFrom = item.post?.resharedFrom ?? null;
  const resharedOriginalName = resharedFrom
    ? (resharedFrom.author.displayName ?? resharedFrom.author.username)
    : null;
  const title = item.article?.title
    ?? (isReshare
      ? (item.post!.content ? textPreview(item.post!.content) : t("resharedPost"))
      : textPreview(item.post?.content ?? ""));
  const content = item.article?.contentMd
    ? textPreview(item.article.contentMd)
    : (!isReshare && item.post)
    ? textPreview(item.post.content)
    : "";
  const articleOwnerUsername = item.article?.board.owner.username.replace(/^@+/, "");
  const detailHref = item.article
    ? `/@${articleOwnerUsername}/${item.article.slug || item.article.id}`
    : item.post
    ? `/posts/${item.post.id}`
    : null;
  const publishedAt = item.article?.publishedAt ?? item.post?.createdAt ?? null;
  const replies = item.post?.replyCount ?? 0;
  const signatureInfo = item.article?.signatureInfo ?? item.post?.signatureInfo;
  const avatarCls = avatarColor(author.username);

  const reactionCounts = item.post?.reactionCounts ?? [];

  return (
    <article className="py-6">
      {/* Meta row */}
      <div className="mb-3 flex items-center gap-2 text-sm">
        <span className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-sm font-semibold ${avatarCls}`}>
          {name.slice(0, 1).toUpperCase()}
        </span>
        <Link href={`/@${author.username}`} className="font-semibold text-[var(--app-text-bright)] hover:text-[var(--app-text-heading)]">
          {name}
        </Link>
        <LevelBadge level={trust} />
        <span className="text-[var(--app-text-dim)]">·</span>
        <span className="text-[var(--app-text-muted)]">{formatDate(publishedAt)}</span>
        {signatureInfo?.isSigned && (
          <SignatureMark
            isVerified={signatureInfo.isVerified}
            trustLevel={author.trustLevel}
            explanation={signatureInfo.explanation}
          />
        )}
      </div>

      {/* Title */}
      <h2
        className="mb-1.5 text-xl font-bold leading-snug text-[var(--app-text-heading)]"
        style={{ fontFamily: "var(--font-playfair), 'Playfair Display', Georgia, serif" }}
      >
        {detailHref ? (
          <Link href={detailHref} className="hover:text-[var(--app-accent)] transition-colors">
            {title}
          </Link>
        ) : (
          title
        )}
      </h2>

      {/* Content preview */}
      {content && (
        detailHref ? (
          <Link href={detailHref} className="block hover:opacity-90">
            <p className="mb-4 text-sm leading-relaxed text-[var(--app-text-secondary)]">{content}</p>
          </Link>
        ) : (
          <p className="mb-4 text-sm leading-relaxed text-[var(--app-text-secondary)]">{content}</p>
        )
      )}

      {/* Embedded reshared post */}
      {isReshare && resharedFrom && (
        <div className="mb-3 rounded-xl border border-[var(--app-border)] bg-[var(--app-surface)] px-4 py-3">
          <div className="mb-1 flex items-center gap-1.5 text-xs text-[var(--app-text-muted)]">
            <span className={`flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-[10px] font-semibold ${avatarColor(resharedFrom.author.username)}`}>
              {resharedOriginalName!.slice(0, 1).toUpperCase()}
            </span>
            <Link href={`/@${resharedFrom.author.username}`} className="font-medium text-[var(--app-text-secondary)] hover:text-[var(--app-text-heading)]">
              {resharedOriginalName}
            </Link>
            <span>·</span>
            <span>{formatDate(resharedFrom.createdAt)}</span>
          </div>
          <p className="text-sm leading-relaxed text-[var(--app-text-secondary)]">
            {textPreview(resharedFrom.content, 200)}
          </p>
        </div>
      )}

      {/* Reaction + action bar */}
      {item.post ? (
        <ReactBar
          postId={item.post.id}
          initialViewerEmotion={item.post.viewerEmotion}
          initialReactionCounts={reactionCounts}
          replyCount={replies}
          replyHref={`/posts/${item.post.id}`}
          postPreview={{ content: item.post.content, authorName: name }}
        />
      ) : (
        <div className="flex divide-x divide-[var(--app-border)] border-t border-[var(--app-border)]">
          {detailHref && (
            <Link
              href={detailHref}
              className="flex flex-1 items-center justify-center gap-1.5 py-2.5 text-sm font-medium text-[var(--app-text-muted)] transition-colors hover:bg-[var(--app-hover-2)] hover:text-[var(--app-text-secondary)]"
            >
              <span>💬</span>
              <span>{t("comment")}</span>
            </Link>
          )}
          <button
            type="button"
            className="flex flex-1 items-center justify-center gap-1.5 py-2.5 text-sm font-medium text-[var(--app-text-muted)] transition-colors hover:bg-[var(--app-hover-2)] hover:text-[var(--app-text-secondary)]"
          >
            <span>↗</span>
            <span>{t("share")}</span>
          </button>
        </div>
      )}
    </article>
  );
}
