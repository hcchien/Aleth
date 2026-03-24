import { notFound } from "next/navigation";
import { getTranslations } from "next-intl/server";
import Link from "next/link";
import { gql } from "@/lib/gql";
import { ForumShell, FanPage } from "../components/forum-shell";
import { SubscribeButton } from "../components/subscribe-button";
import { FollowButton } from "../components/follow-button";
import { BoardSettingsLink } from "../components/board-settings-link";

const BOARD_QUERY = `
  query Board($username: String!) {
    board(username: $username) {
      id name description subscriberCount isSubscribed defaultAccess createdAt
      owner { id username displayName trustLevel createdAt }
      series { id title description articleCount }
    }
  }
`;

const BOARD_ARTICLES_QUERY = `
  query BoardArticles($username: String!, $limit: Int) {
    boardArticles(username: $username, limit: $limit) {
      items { id title slug publishedAt author { username displayName } }
      nextCursor
      hasMore
    }
  }
`;

const FOLLOW_STATS_QUERY = `
  query FollowStats($userID: ID!) {
    followStats(userID: $userID) {
      followerCount
      followingCount
    }
  }
`;

interface SeriesItem {
  id: string;
  title: string;
  description: string | null;
  articleCount: number;
}

interface Board {
  id: string;
  name: string;
  description: string | null;
  subscriberCount: number;
  isSubscribed: boolean;
  defaultAccess: string;
  createdAt: string;
  series: SeriesItem[];
  owner: {
    id: string;
    username: string;
    displayName: string | null;
    trustLevel: number;
    createdAt: string;
  };
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

interface FollowStats {
  followerCount: number;
  followingCount: number;
}

interface PageProps {
  params: Promise<{ username: string }>;
}

// Avatar color palette — deterministic by first char
const AVATAR_COLORS = [
  "bg-blue-500/20 text-blue-400",
  "bg-violet-500/20 text-violet-400",
  "bg-emerald-500/20 text-emerald-400",
  "bg-amber-500/20 text-amber-400",
  "bg-rose-500/20 text-rose-400",
  "bg-cyan-500/20 text-cyan-400",
];

function avatarColor(name: string) {
  return AVATAR_COLORS[name.charCodeAt(0) % AVATAR_COLORS.length];
}

// Trust level badge config — Stitch style
const TRUST_CONFIG: Record<number, { label: string; cls: string }> = {
  0: { label: "Unverified",      cls: "text-[var(--app-text-muted)] bg-[var(--app-surface-2)]" },
  1: { label: "L1 Member",       cls: "text-cyan-400 bg-cyan-400/10" },
  2: { label: "Verified",        cls: "text-[var(--app-secondary)] bg-[var(--app-secondary-bg)]" },
  3: { label: "Senior Analyst",  cls: "text-amber-400 bg-amber-400/10" },
  4: { label: "Core Validator",  cls: "text-purple-400 bg-purple-400/10" },
};

function TrustPill({ level }: { level: number }) {
  const cfg = TRUST_CONFIG[Math.min(4, Math.max(0, level))] ?? TRUST_CONFIG[0];
  return (
    <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-tighter ${cfg.cls}`}>
      <svg width="11" height="11" viewBox="0 0 8 8" fill="currentColor" aria-hidden="true">
        <path d="M3.5 6.5L1 4l.7-.7 1.8 1.8 3.8-3.8.7.7z" />
      </svg>
      {cfg.label}
    </span>
  );
}

function StatItem({ value, label }: { value: number; label: string }) {
  return (
    <div className="flex flex-col items-center">
      <span className="text-lg font-bold text-[var(--app-text-heading)]" style={{ fontFamily: "var(--font-newsreader), serif" }}>
        {value.toLocaleString()}
      </span>
      <span className="text-[10px] font-bold uppercase tracking-widest text-[var(--app-text-muted)]">{label}</span>
    </div>
  );
}

function formatDate(iso: string | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleDateString("zh-TW", { year: "numeric", month: "numeric", day: "numeric" });
}

export default async function UserProfilePage({ params }: PageProps) {
  const t = await getTranslations("profile");
  const tSeries = await getTranslations("series");
  const { username: rawUsername } = await params;
  const username = rawUsername.startsWith("%40")
    ? rawUsername.slice(3)
    : rawUsername.startsWith("@")
    ? rawUsername.slice(1)
    : rawUsername;

  let board: Board | null = null;
  let articles: ArticleConnection = { items: [], nextCursor: null, hasMore: false };
  let followStats: FollowStats | null = null;

  try {
    const boardData = await gql<{ board: Board | null }>(
      BOARD_QUERY,
      { username },
      { revalidate: 60 }
    );
    board = boardData.board;
  } catch {
    // gateway not available
  }

  if (!board) notFound();

  const [articlesResult, followStatsResult] = await Promise.allSettled([
    gql<{ boardArticles: ArticleConnection }>(
      BOARD_ARTICLES_QUERY,
      { username, limit: 20 },
      { revalidate: 60 }
    ),
    gql<{ followStats: FollowStats }>(
      FOLLOW_STATS_QUERY,
      { userID: board.owner.id },
      { revalidate: 60 }
    ),
  ]);
  if (articlesResult.status === "fulfilled") articles = articlesResult.value.boardArticles;
  if (followStatsResult.status === "fulfilled") followStats = followStatsResult.value.followStats;

  const enrichedArticles = articles.items.map((a) => ({
    ...a,
    board: { name: board!.name, owner: { username: board!.owner.username } },
  }));

  const fanPage: FanPage = {
    id: board.id,
    icon: "◁",
    ownerUsername: board.owner.username,
    name: board.name,
    count: board.subscriberCount,
  };

  const displayName = board.owner.displayName ?? board.owner.username;
  const avatarChar = (displayName[0] || "A").toUpperCase();
  const avatarCls = avatarColor(displayName);
  const joinedDate = (() => {
    const d = new Date(board.owner.createdAt);
    if (Number.isNaN(d.getTime())) return "";
    return d.toLocaleDateString(undefined, { year: "numeric", month: "long" });
  })();

  return (
    <ForumShell fanPages={[fanPage]}>

      {/* ── Profile Hero ── */}
      <div className="mb-10">

        {/* Banner strip */}
        <div className="h-24 rounded-2xl mb-0 bg-gradient-to-r from-[var(--app-accent)]/20 via-[var(--app-secondary)]/10 to-transparent border border-[var(--app-border-inner)] rounded-b-none" />

        {/* Identity card */}
        <div className="rounded-2xl rounded-t-none border border-t-0 border-[var(--app-border-inner)] bg-[var(--app-surface-3)] px-6 pb-6">

          {/* Avatar row */}
          <div className="flex items-end justify-between -mt-10 mb-4">
            <div className={`flex h-20 w-20 shrink-0 items-center justify-center rounded-2xl text-3xl font-bold border-4 border-[var(--app-bg)] ${avatarCls}`}>
              {avatarChar}
            </div>

            {/* Action buttons — top-right */}
            <div className="flex items-center gap-2 mt-10">
              <FollowButton
                userID={board.owner.id}
                ownerUsername={board.owner.username}
                initialFollowerCount={followStats?.followerCount ?? 0}
              />
              <SubscribeButton
                ownerID={board.owner.id}
                initialSubscribed={board.isSubscribed}
                initialCount={board.subscriberCount}
              />
              <BoardSettingsLink ownerUsername={board.owner.username} />
            </div>
          </div>

          {/* Name + handle */}
          <div className="mb-3">
            <div className="flex items-center gap-2 flex-wrap">
              <h1 className="text-2xl font-black text-[var(--app-text-heading)] tracking-tight" style={{ fontFamily: "var(--font-manrope), 'Manrope', sans-serif" }}>
                {displayName}
              </h1>
              <TrustPill level={board.owner.trustLevel} />
            </div>
            <p className="text-sm text-[var(--app-text-muted)] mt-0.5">
              @{board.owner.username}
              {joinedDate && (
                <span className="ml-3 text-[var(--app-text-dim)]">· {t("joinedDate", { date: joinedDate })}</span>
              )}
            </p>
          </div>

          {/* Bio */}
          {board.description && (
            <p className="mb-4 max-w-lg text-sm leading-relaxed text-[var(--app-text-secondary)]">
              {board.description}
            </p>
          )}

          {/* Stats */}
          <div className="flex items-center gap-6 border-t border-[var(--app-border-inner)] pt-4">
            <StatItem value={articles.items.length} label={t("articles")} />
            <div className="w-px h-8 bg-[var(--app-border-inner)]" />
            <StatItem value={board.subscriberCount} label={t("subscribers")} />
            {followStats !== null && (
              <>
                <div className="w-px h-8 bg-[var(--app-border-inner)]" />
                <StatItem value={followStats.followerCount} label={t("followers")} />
                <div className="w-px h-8 bg-[var(--app-border-inner)]" />
                <StatItem value={followStats.followingCount} label="Following" />
              </>
            )}
          </div>
        </div>
      </div>

      {/* ── Series ── */}
      {board.series.length > 0 && (
        <div className="mb-10">
          <h2 className="mb-4 text-[10px] font-bold uppercase tracking-widest text-[var(--app-text-muted)]">
            {tSeries("seriesSection")}
          </h2>
          <div className="grid gap-3 sm:grid-cols-2">
            {board.series.map((s) => (
              <Link
                key={s.id}
                href={`/@${username}/series/${s.id}`}
                className="group flex flex-col gap-1 rounded-xl border border-[var(--app-border-inner)] bg-[var(--app-surface-2)] p-5 hover:border-[var(--app-accent-border)] hover:bg-[var(--app-hover)] transition-all"
              >
                <div className="flex items-start justify-between gap-2">
                  <p className="font-bold text-[var(--app-text-heading)] group-hover:text-[var(--app-accent)] transition-colors leading-snug">
                    {s.title}
                  </p>
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="shrink-0 mt-0.5 text-[var(--app-text-muted)] group-hover:text-[var(--app-accent)] transition-colors" aria-hidden="true">
                    <path d="M5 12h14M12 5l7 7-7 7" />
                  </svg>
                </div>
                {s.description && (
                  <p className="text-xs text-[var(--app-text-muted)] line-clamp-2 leading-relaxed">
                    {s.description}
                  </p>
                )}
                <p className="mt-1 text-[10px] font-bold uppercase tracking-widest text-[var(--app-text-dim)]">
                  {tSeries("articleCount", { count: s.articleCount })}
                </p>
              </Link>
            ))}
          </div>
        </div>
      )}

      {/* ── Articles ── */}
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-[10px] font-bold uppercase tracking-widest text-[var(--app-text-muted)]">
          {t("articles")}
          <span className="ml-2 text-[var(--app-text-dim)]">
            {enrichedArticles.length}{articles.hasMore ? "+" : ""}
          </span>
        </h2>
      </div>

      {enrichedArticles.length === 0 ? (
        <div className="rounded-xl border border-[var(--app-border-inner)] bg-[var(--app-surface-3)] px-6 py-16 text-center">
          <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" strokeLinejoin="round" className="mx-auto mb-3 text-[var(--app-text-dim)]" aria-hidden="true">
            <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z" /><polyline points="14 2 14 8 20 8" />
          </svg>
          <p className="text-sm text-[var(--app-text-muted)]">{t("noArticles")}</p>
        </div>
      ) : (
        <div className="space-y-0">
          {enrichedArticles.map((article, i) => {
            const ownerUsername = article.board.owner.username.replace(/^@+/, "");
            const articleRef = article.slug || article.id;
            const href = `/@${ownerUsername}/${articleRef}`;
            const date = formatDate(article.publishedAt);
            const isLast = i === enrichedArticles.length - 1;

            return (
              <Link
                key={article.id}
                href={href}
                className={`group relative flex gap-5 py-6 ${!isLast ? "border-b border-[var(--app-border-inner)]" : ""}`}
              >
                {/* Index number */}
                <span className="hidden md:flex w-6 shrink-0 items-start justify-end pt-1 text-[11px] font-mono text-[var(--app-text-dim)] select-none">
                  {String(i + 1).padStart(2, "0")}
                </span>

                <div className="flex-1 min-w-0">
                  <h3
                    className="text-xl font-bold leading-snug text-[var(--app-text-heading)] group-hover:text-[var(--app-accent)] transition-colors mb-2"
                    style={{ fontFamily: "var(--font-newsreader), 'Newsreader', serif" }}
                  >
                    {article.title}
                  </h3>

                  {/* Meta */}
                  <div className="flex flex-wrap items-center gap-1.5 text-[11px] font-bold uppercase tracking-wider text-[var(--app-text-muted)]">
                    <span className="text-[var(--app-accent)] opacity-70">{article.board.name}</span>
                    {date && (
                      <>
                        <span className="text-[var(--app-text-dim)]">·</span>
                        <span>{date}</span>
                      </>
                    )}
                  </div>
                </div>

                {/* Arrow indicator */}
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="shrink-0 self-center text-[var(--app-text-dim)] opacity-0 group-hover:opacity-100 group-hover:text-[var(--app-accent)] transition-all -translate-x-1 group-hover:translate-x-0" aria-hidden="true">
                  <path d="M5 12h14M12 5l7 7-7 7" />
                </svg>
              </Link>
            );
          })}
        </div>
      )}

    </ForumShell>
  );
}
