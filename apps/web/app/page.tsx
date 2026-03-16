import { gql } from "@/lib/gql";
import { FanPage, FollowedUser, ForumShell } from "./components/forum-shell";
import { HomeFeedClient, FeedItem } from "./components/home-feed-client";

const EXPLORE_FEED_QUERY = `
  query ExploreFeed($limit: Int) {
    exploreFeed(limit: $limit) {
      items {
        id
        type
        post {
          id content replyCount likeCount viewerEmotion
          reactionCounts { emotion count }
          createdAt resharedFromId
          resharedFrom { id content createdAt author { id username displayName trustLevel } }
          signatureInfo { isSigned isVerified explanation }
          author { id username displayName trustLevel }
        }
        article {
          id slug title contentMd publishedAt
          signatureInfo { isSigned isVerified explanation }
          author { id username displayName trustLevel }
          board { id name subscriberCount owner { username } }
        }
      }
      nextCursor
      hasMore
    }
  }
`;

interface ExploreFeedResponse {
  exploreFeed: {
    items: FeedItem[];
    nextCursor: string | null;
    hasMore: boolean;
  };
}

function tierClass(level: number): string {
  if (level >= 4) return "border-amber-500/40 bg-amber-500/15 text-amber-300";
  if (level >= 3) return "border-orange-500/40 bg-orange-500/15 text-orange-300";
  if (level >= 2) return "border-lime-500/40 bg-lime-500/15 text-lime-300";
  return "border-cyan-500/40 bg-cyan-500/15 text-cyan-300";
}

export default async function HomePage() {
  let feedItems: FeedItem[] = [];
  let nextCursor: string | null = null;
  let hasMore = false;

  try {
    const data = await gql<ExploreFeedResponse>(
      EXPLORE_FEED_QUERY,
      { limit: 20 },
      { revalidate: 30, tags: ["feed:explore"] }
    );
    feedItems = data.exploreFeed.items;
    nextCursor = data.exploreFeed.nextCursor;
    hasMore = data.exploreFeed.hasMore;
  } catch {
    feedItems = [];
  }

  // Build sidebar widgets from server-rendered explore feed
  const users = new Map<string, FollowedUser>();
  const pages = new Map<string, FanPage>();
  for (const item of feedItems) {
    const author = item.post?.author ?? item.article?.author;
    if (author && !users.has(author.id) && users.size < 3) {
      users.set(author.id, {
        id: author.id,
        initial: (author.displayName ?? author.username).slice(0, 1).toUpperCase(),
        name: author.displayName ?? author.username,
        username: author.username,
        tier: `L${author.trustLevel}`,
        tierClass: tierClass(author.trustLevel),
        trustLevel: author.trustLevel,
      });
    }

    const board = item.article?.board;
    if (board && !pages.has(board.id) && pages.size < 3) {
      pages.set(board.id, {
        id: board.id,
        icon: "◁",
        ownerUsername: board.owner.username,
        name: board.name,
        count: board.subscriberCount,
      });
    }
  }

  return (
    <ForumShell followedUsers={[...users.values()]} fanPages={[...pages.values()]}>
      <HomeFeedClient
        initialItems={feedItems}
        initialCursor={nextCursor}
        initialHasMore={hasMore}
      />
    </ForumShell>
  );
}
