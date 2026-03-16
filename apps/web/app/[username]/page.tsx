import { notFound } from "next/navigation";
import { getTranslations } from "next-intl/server";
import { gql } from "@/lib/gql";
import { ForumShell, FanPage } from "../components/forum-shell";
import { ArticleCard } from "../components/article-card";
import { SubscribeButton } from "../components/subscribe-button";
import { FollowButton } from "../components/follow-button";
import { TrustBadge } from "../components/trust-badge";
import { BoardSettingsLink } from "../components/board-settings-link";

const BOARD_QUERY = `
  query Board($username: String!) {
    board(username: $username) {
      id name description subscriberCount isSubscribed defaultAccess createdAt
      owner { id username displayName trustLevel createdAt }
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

interface Board {
  id: string;
  name: string;
  description: string | null;
  subscriberCount: number;
  isSubscribed: boolean;
  defaultAccess: string;
  createdAt: string;
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

export default async function UserProfilePage({ params }: PageProps) {
  const t = await getTranslations("profile");
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

  if (!board) {
    notFound();
  }

  // Fetch articles and follow stats in parallel
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
  const joinedDate = (() => {
    const d = new Date(board.owner.createdAt);
    if (Number.isNaN(d.getTime())) return "";
    return d.toLocaleDateString(undefined, { year: "numeric", month: "long" });
  })();

  return (
    <ForumShell fanPages={[fanPage]}>
      {/* Profile hero */}
      <div className="mb-8 rounded-2xl border border-[var(--app-border-2)] bg-[var(--app-surface)] p-6">
        <div className="flex items-start justify-between gap-4">
          <div className="flex gap-5">
            {/* Avatar */}
            <div className="flex h-20 w-20 shrink-0 items-center justify-center rounded-full bg-[var(--app-accent-bg)] text-3xl font-semibold text-[var(--app-accent)]">
              {avatarChar}
            </div>

            {/* Info */}
            <div className="min-w-0">
              <h1 className="font-serif text-2xl text-[var(--app-text-heading)]">{displayName}</h1>
              <p className="text-sm text-[var(--app-text-muted)]">@{board.owner.username}</p>

              {/* Stats row */}
              <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-[var(--app-text-secondary)]">
                <TrustBadge level={board.owner.trustLevel} />
                <span className="text-[var(--app-text-dim)]">·</span>
                <span>
                  <span className="font-medium text-[var(--app-text-bright)]">{board.subscriberCount}</span>{" "}
                  {t("subscribers")}
                </span>
                {followStats !== null && (
                  <>
                    <span className="text-[var(--app-text-dim)]">·</span>
                    <span>
                      <span className="font-medium text-[var(--app-text-bright)]">{followStats.followerCount}</span>{" "}
                      {t("followers")}
                    </span>
                  </>
                )}
                {joinedDate && (
                  <>
                    <span className="text-[var(--app-text-dim)]">·</span>
                    <span className="text-[var(--app-text-muted)]">{t("joinedDate", { date: joinedDate })}</span>
                  </>
                )}
              </div>

              {/* Bio / board description */}
              {board.description && (
                <p className="mt-3 max-w-lg text-sm leading-relaxed text-[var(--app-text-bright)]">
                  {board.description}
                </p>
              )}
            </div>
          </div>

          {/* Follow + Subscribe + board settings */}
          <div className="shrink-0 flex flex-col items-end gap-2">
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
      </div>

      {/* Articles section */}
      <div className="mb-4 flex items-center justify-between">
        <h2 className="font-serif text-xl text-[var(--app-text-bright)]">
          {t("articles")}
          <span className="ml-2 font-mono text-sm text-[var(--app-text-muted)]">
            ({enrichedArticles.length}{articles.hasMore ? "+" : ""})
          </span>
        </h2>
      </div>

      {enrichedArticles.length === 0 ? (
        <div className="rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)] px-6 py-10 text-center text-sm text-[var(--app-text-muted)]">
          {t("noArticles")}
        </div>
      ) : (
        <div className="space-y-4">
          {enrichedArticles.map((article) => (
            <ArticleCard key={article.id} article={article} />
          ))}
        </div>
      )}
    </ForumShell>
  );
}
