"use client";

import { useState, useEffect, useRef } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useAuth } from "@/lib/auth-context";
import { gqlClient } from "@/lib/gql-client";
import Image from "next/image";
import { ReactBar } from "./react-bar";
import { ComposeBox } from "./compose-box";
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
  imageUrls: string[];
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
  id content imageUrls replyCount likeCount viewerEmotion
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

const MY_REMOTE_FEED_QUERY = `
  query MyRemoteFeed($limit: Int, $before: String) {
    myRemoteFeed(limit: $limit, before: $before) {
      posts {
        id actorURL handle content publishedAt
      }
      hasMore
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

// ─── VerifiedBadge ────────────────────────────────────────────────────────────

function VerifiedBadge({ level }: { level: number }) {
  if (level < 2) return null;
  return (
    <span className="inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-tighter bg-[var(--app-verified-bg)] text-[var(--app-verified)] border border-[var(--app-verified-border)]">
      <svg width="10" height="10" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
        <path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z" />
      </svg>
      Verified Identity
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

interface RemotePostItem {
  id: string;
  actorURL: string;
  handle: string;
  content: string;
  publishedAt: string;
}

interface RemotePostConnection {
  posts: RemotePostItem[];
  hasMore: boolean;
}

// ─── NetworkSidebar ───────────────────────────────────────────────────────────

export function NetworkSidebar({ userTrustLevel = 0 }: { userTrustLevel?: number }) {
  // Identity completeness: rough estimate based on trust level
  const identityPct = Math.min(100, userTrustLevel * 25);

  const trendingTags = [
    "#DecentralizedEnergy",
    "#LedgerSafety",
    "#GlobalLiquidity",
    "#VerifiedSources",
    "#ChainGovernance",
  ];

  return (
    <aside className="w-[240px] shrink-0 space-y-4">
      {/* Elevate Your Trust card */}
      <div className="rounded-xl border border-[var(--app-border)] bg-[var(--app-surface)] p-4">
        <h3 className="mb-1 text-sm font-bold text-[var(--app-text-heading)]">Elevate Your Trust</h3>
        <p className="mb-3 text-xs text-[var(--app-text-secondary)]">
          Complete identity certification to unlock full network access.
        </p>
        {/* Progress bar */}
        <div className="mb-1 flex items-center justify-between text-[10px] font-semibold uppercase tracking-wide text-[var(--app-text-muted)]">
          <span>Identity Completeness</span>
          <span>{identityPct}%</span>
        </div>
        <div className="mb-4 h-1.5 w-full rounded-full bg-[var(--app-surface-2)]">
          <div
            className="h-1.5 rounded-full bg-[var(--app-accent)] transition-all"
            style={{ width: `${identityPct}%` }}
          />
        </div>
        <Link href="/settings" className="btn-primary w-full justify-center text-xs">
          Certify Identity
        </Link>
      </div>

      {/* Network stats */}
      <div className="rounded-xl border border-[var(--app-border)] bg-[var(--app-surface)] p-4 space-y-3">
        <h3 className="text-xs font-bold uppercase tracking-widest text-[var(--app-text-muted)]">Network Health</h3>
        {/* Metric: health */}
        <div className="border-l-[3px] border-[var(--app-verified)] pl-3">
          <div className="text-lg font-bold text-[var(--app-text-heading)]">99.98%</div>
          <div className="text-[11px] text-[var(--app-text-secondary)]">Nominal performance</div>
        </div>
        {/* Metric: validators */}
        <div className="border-l-[3px] border-[var(--app-accent)] pl-3">
          <div className="text-lg font-bold text-[var(--app-text-heading)]">14,202</div>
          <div className="text-[11px] text-[var(--app-text-secondary)]">Active Validators</div>
        </div>
      </div>

      {/* Trending intelligence */}
      <div className="rounded-xl border border-[var(--app-border)] bg-[var(--app-surface)] p-4">
        <h3 className="mb-3 text-xs font-bold uppercase tracking-widest text-[var(--app-text-muted)]">Trending Intelligence</h3>
        <div className="flex flex-wrap gap-1.5">
          {trendingTags.map((tag) => (
            <span key={tag} className="badge-protocol cursor-pointer hover:opacity-80 transition-opacity">
              {tag}
            </span>
          ))}
        </div>
      </div>

      {/* Footer links */}
      <div className="flex flex-wrap gap-x-3 gap-y-1 px-1 text-[10px] text-[var(--app-text-muted)]">
        {["API", "NODES", "SECURITY", "TRANSPARENCY"].map((item) => (
          <a key={item} href="#" className="hover:text-[var(--app-text-secondary)] transition-colors">
            {item}
          </a>
        ))}
      </div>
    </aside>
  );
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
  const [activeTab, setActiveTab] = useState<"local" | "fediverse">("local");
  const [remotePosts, setRemotePosts] = useState<RemotePostItem[]>([]);
  const [remoteHasMore, setRemoteHasMore] = useState(false);
  const [remoteBefore, setRemoteBefore] = useState<string | null>(null);
  const [remoteLoading, setRemoteLoading] = useState(false);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState(false);
  const fetchedForUserRef = useRef<string | undefined>(undefined);

  // Once auth resolves, fetch personalized feed if logged in.
  useEffect(() => {
    if (authLoading) return;
    if (!user) return;
    if (fetchedForUserRef.current === user.id) return;
    fetchedForUserRef.current = user.id;

    gqlClient<{ feed: FeedConnection }>(PERSONALIZED_FEED_QUERY, { limit: 20 })
      .then((data) => {
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

  async function fetchFediverse(before?: string | null) {
    if (!user?.apEnabled) return;
    setRemoteLoading(true);
    try {
      const data = await gqlClient<{ myRemoteFeed: RemotePostConnection }>(
        MY_REMOTE_FEED_QUERY,
        { limit: 20, ...(before ? { before } : {}) }
      );
      const posts = data.myRemoteFeed.posts ?? [];
      if (before) {
        setRemotePosts((prev) => [...prev, ...posts]);
      } else {
        setRemotePosts(posts);
      }
      setRemoteHasMore(data.myRemoteFeed.hasMore);
      if (posts.length > 0) {
        setRemoteBefore(posts[posts.length - 1].publishedAt);
      }
    } catch {
      // silently fail
    } finally {
      setRemoteLoading(false);
    }
  }

  function loadFediverse() { fetchFediverse(); }
  function loadMoreFediverse() { if (!remoteLoading && remoteHasMore) fetchFediverse(remoteBefore); }

  async function loadMore() {
    if (loading || !hasMore) return;
    setLoading(true);
    setLoadError(false);
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
      setLoadError(true);
    } finally {
      setLoading(false);
    }
  }

  const headingText = feedType === "personalized" ? t("myFeed") : t("allFeed");
  const showFediverseTab = !authLoading && !!user?.apEnabled;

  function handleFediverseTab() {
    setActiveTab("fediverse");
    if (remotePosts.length === 0 && !remoteLoading) {
      loadFediverse();
    }
  }

  return (
    <>
      {/* Feed heading */}
      <div className="mb-12 flex justify-between items-end">
        <div>
          <h1 className="font-headline text-4xl font-extrabold text-[var(--app-text-heading)] tracking-tight leading-none mb-2">
            {activeTab === "fediverse" ? "Fediverse" : "Network Feed"}
          </h1>
          <p className="text-[var(--app-text-muted)] max-w-md text-sm">
            {activeTab === "fediverse"
              ? "Posts from the fediverse"
              : "Access real-time intelligence verified through decentralized consensus."}
          </p>
        </div>
        <div className="flex gap-2">
          <button
            type="button"
            className="p-3 bg-[var(--app-surface-2)] rounded-lg text-[var(--app-accent)] flex items-center gap-2 hover:bg-[var(--app-surface-4)] transition-all"
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
              <path d="M3 6h18M7 12h10M11 18h2" strokeLinecap="round" />
            </svg>
            <span className="text-[10px] font-bold uppercase tracking-widest">Filter</span>
          </button>
        </div>
      </div>

      {showFediverseTab && (
        <div className="mb-5 flex gap-1 border-b border-[var(--app-border)]">
          <button
            onClick={() => setActiveTab("local")}
            className={[
              "px-3 pb-2 text-sm font-medium transition-colors",
              activeTab === "local"
                ? "border-b-2 border-[var(--app-accent)] text-[var(--app-accent)]"
                : "text-[var(--app-text-muted)] hover:text-[var(--app-text-secondary)]",
            ].join(" ")}
          >
            {headingText}
          </button>
          <button
            onClick={handleFediverseTab}
            className={[
              "px-3 pb-2 text-sm font-medium transition-colors",
              activeTab === "fediverse"
                ? "border-b-2 border-[var(--app-accent)] text-[var(--app-accent)]"
                : "text-[var(--app-text-muted)] hover:text-[var(--app-text-secondary)]",
            ].join(" ")}
          >
            Fediverse
          </button>
        </div>
      )}

      {activeTab === "local" && (
        <>
          <ComposeBox />

          {items.length === 0 && !authLoading ? (
            <div className="rounded-2xl border border-[var(--app-border)] bg-[var(--app-surface-3)] px-6 py-10 text-center text-sm text-[var(--app-text-secondary)]">
              {feedType === "personalized" ? t("emptyPersonalized") : t("emptyExplore")}
            </div>
          ) : (
            <div className="space-y-16">
              {items.map((item) => (
                <FeedCard key={item.id} item={item} />
              ))}
            </div>
          )}

          {loadError && (
            <div className="py-4 text-center">
              <p className="mb-2 text-sm text-[var(--app-text-muted)]">Failed to load posts.</p>
              <button onClick={loadMore} className="text-sm text-[var(--app-accent)] hover:underline">
                Retry
              </button>
            </div>
          )}

          {hasMore && !loadError && (
            <div className="py-6 text-center">
              <button
                onClick={loadMore}
                disabled={loading}
                className="btn-ghost disabled:opacity-50"
              >
                {loading ? tCommon("loading") : t("loadMore")}
              </button>
            </div>
          )}
        </>
      )}

      {activeTab === "fediverse" && (
        <>
          {remoteLoading && remotePosts.length === 0 && (
            <div className="py-10 text-center text-sm text-[var(--app-text-muted)]">
              {tCommon("loading")}
            </div>
          )}

          {!remoteLoading && remotePosts.length === 0 && (
            <div className="rounded-xl border border-[var(--app-border)] bg-[var(--app-surface)] px-6 py-10 text-center text-sm text-[var(--app-text-secondary)]">
              <p className="mb-2 font-medium text-[var(--app-text-heading)]">No fediverse posts yet</p>
              <p className="text-[var(--app-text-muted)]">
                Go to{" "}
                <Link href="/settings" className="text-[var(--app-accent)] hover:underline">
                  Settings → ActivityPub
                </Link>{" "}
                and follow a Threads or Mastodon account to see posts here.
              </p>
            </div>
          )}

          {remotePosts.length > 0 && (
            <div className="space-y-16">
              {remotePosts.map((post) => (
                <RemotePostCard key={post.id} post={post} />
              ))}
            </div>
          )}

          {remoteHasMore && (
            <div className="py-6 text-center">
              <button
                onClick={loadMoreFediverse}
                disabled={remoteLoading}
                className="btn-ghost disabled:opacity-50"
              >
                {remoteLoading ? tCommon("loading") : t("loadMore")}
              </button>
            </div>
          )}
        </>
      )}
    </>
  );
}

// ─── RemotePostCard ────────────────────────────────────────────────────────────

function RemotePostCard({ post }: { post: RemotePostItem }) {
  const initial = post.handle.replace(/^@/, "").charAt(0).toUpperCase();
  const color = avatarColor(post.handle);
  const date = formatDate(post.publishedAt);
  const plainContent = post.content.replace(/<[^>]+>/g, " ").replace(/\s+/g, " ").trim();

  return (
    <div className="post-card border-b border-[var(--app-border)] last:border-b-0">
      <div className="mb-2 flex items-center gap-2">
        <span className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-sm font-semibold ${color}`}>
          {initial}
        </span>
        <div className="min-w-0">
          <a
            href={post.actorURL}
            target="_blank"
            rel="noopener noreferrer"
            className="text-sm font-medium text-[var(--app-text)] hover:underline font-mono"
          >
            {post.handle}
          </a>
          {date && (
            <span className="ml-2 text-xs text-[var(--app-text-muted)]">{date}</span>
          )}
        </div>
        <span className="ml-auto shrink-0 rounded-full border border-[var(--app-border-2)] px-2 py-0.5 text-[0.6rem] font-semibold uppercase tracking-widest text-[var(--app-text-dim)]">
          Fediverse
        </span>
      </div>
      <p className="text-sm leading-relaxed text-[var(--app-text-secondary)]">
        {textPreview(plainContent, 280)}
      </p>
    </div>
  );
}

// ─── SignatureMark ────────────────────────────────────────────────────────────

function signatureColor(trustLevel: number): string {
  if (trustLevel >= 3) return "text-amber-400";
  if (trustLevel === 2) return "text-[var(--app-verified)]";
  return "text-[var(--app-accent)]";
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
  const boardName = item.article?.board.name;
  const reactionCounts = item.post?.reactionCounts ?? [];

  return (
    <article className="relative group post-card">
      {/* Timeline connector line — runs down to the next post */}
      <div
        className="absolute hidden md:block w-px bg-[var(--app-border-inner)] rounded-full"
        style={{ left: "27px", top: "72px", bottom: "-4rem" }}
        aria-hidden="true"
      >
        <div className="absolute bottom-0 left-1/2 -translate-x-1/2 w-1.5 h-1.5 rounded-full bg-[var(--app-secondary)]" />
      </div>

      <div className="flex gap-3 sm:gap-6">
        {/* Square rounded avatar */}
        <div className="flex-shrink-0">
          <span
            className={`flex h-10 w-10 sm:h-14 sm:w-14 items-center justify-center rounded-xl text-base sm:text-xl font-bold transition-all duration-500 grayscale opacity-70 group-hover:grayscale-0 group-hover:opacity-100 ${avatarCls}`}
          >
            {name.slice(0, 1).toUpperCase()}
          </span>
        </div>

        <div className="flex-1 min-w-0">
          {/* Meta row */}
          <div className="flex flex-wrap items-center gap-2 mb-2">
            <Link href={`/@${author.username}`} className="font-bold text-[var(--app-text-heading)] text-sm hover:text-[var(--app-accent)] transition-colors">
              {name}
            </Link>
            <VerifiedBadge level={author.trustLevel} />
            {signatureInfo?.isSigned && (
              <SignatureMark
                isVerified={signatureInfo.isVerified}
                trustLevel={author.trustLevel}
                explanation={signatureInfo.explanation}
              />
            )}
            <span className="text-[var(--app-text-dim)] text-[11px]">
              · {formatDate(publishedAt)}
              {boardName && (
                <> in <span className="text-[var(--app-accent)] hover:underline cursor-pointer">{boardName}</span></>
              )}
            </span>
          </div>

          {/* Title: Newsreader serif headline */}
          <h2
            className="font-headline text-2xl font-bold text-[var(--app-text-heading)] mb-3 leading-tight group-hover:text-[var(--app-accent)] transition-colors"
            style={{ textWrap: "balance" as const }}
          >
            {detailHref ? (
              <Link href={detailHref}>{title}</Link>
            ) : (
              title
            )}
          </h2>

          {/* Content preview */}
          {content && (
            <p className="text-[var(--app-text-secondary)] leading-relaxed text-sm mb-4 max-w-2xl">
              {content}
            </p>
          )}

          {/* Image grid */}
          {item.post?.imageUrls && item.post.imageUrls.length > 0 && (
            <div
              className={`mb-4 grid gap-1.5 overflow-hidden rounded-xl ${
                item.post.imageUrls.length === 1
                  ? "grid-cols-1"
                  : item.post.imageUrls.length === 2
                  ? "grid-cols-2"
                  : item.post.imageUrls.length === 3
                  ? "grid-cols-2"
                  : "grid-cols-2"
              }`}
            >
              {item.post.imageUrls.map((url, i) => {
                const isLarge = item.post!.imageUrls.length === 3 && i === 0;
                return (
                  <div
                    key={url}
                    className={`relative overflow-hidden bg-[var(--app-surface-4)] rounded-lg ${isLarge ? "row-span-2" : ""}`}
                    style={{ aspectRatio: item.post!.imageUrls.length === 1 ? "16/9" : "1" }}
                  >
                    <Image
                      src={url}
                      alt={`Image ${i + 1}`}
                      fill
                      className="object-cover hover:scale-105 transition-transform duration-300"
                      unoptimized
                    />
                  </div>
                );
              })}
            </div>
          )}

          {/* Embedded reshared post */}
          {isReshare && resharedFrom && (
            <div className="mb-4 bg-[var(--app-surface-3)] p-4 rounded-xl border-l-4 border-[var(--app-secondary-border)]">
              <div className="mb-1 flex items-center gap-1.5 text-xs text-[var(--app-text-muted)]">
                <span className={`flex h-5 w-5 shrink-0 items-center justify-center rounded text-[10px] font-semibold ${avatarColor(resharedFrom.author.username)}`}>
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

          {/* Action bar */}
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
            <div className="flex items-center gap-6">
              {detailHref && (
                <Link
                  href={detailHref}
                  className="flex items-center gap-1.5 text-[var(--app-text-muted)] hover:text-[var(--app-accent)] transition-colors"
                >
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
                    <path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z" strokeLinecap="round" strokeLinejoin="round" />
                  </svg>
                  <span className="text-[10px] font-bold tracking-wider uppercase">{t("comment")}</span>
                </Link>
              )}
              <button
                type="button"
                className="flex items-center gap-1.5 text-[var(--app-text-muted)] hover:text-[var(--app-text-heading)] transition-colors ml-auto"
              >
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
                  <path d="M4 12v8a2 2 0 002 2h12a2 2 0 002-2v-8M16 6l-4-4-4 4M12 2v13" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </button>
            </div>
          )}
        </div>
      </div>
    </article>
  );
}
