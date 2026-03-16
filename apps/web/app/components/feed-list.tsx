"use client";

import { useState } from "react";
import { gqlClient } from "@/lib/gql-client";
import { PostCard } from "./post-card";
import { ArticleCard } from "./article-card";

interface Author {
  id: string;
  username: string;
  displayName: string | null;
}

interface Post {
  id: string;
  content: string;
  likeCount: number;
  replyCount: number;
  isLiked: boolean;
  viewerEmotion?: string | null;
  reactionCounts: { emotion: string; count: number }[];
  createdAt: string;
  author: Author;
}

interface Article {
  id: string;
  title: string;
  slug: string;
  publishedAt: string | null;
  author: { username: string; displayName: string | null };
  board: { name: string; owner: { username: string } };
}

interface FeedItem {
  id: string;
  type: string;
  score: number;
  post: Post | null;
  article: Article | null;
}

interface FeedConnection {
  items: FeedItem[];
  nextCursor: string | null;
  hasMore: boolean;
}

const EXPLORE_FEED_QUERY = `
  query ExploreFeed($after: String, $limit: Int) {
    exploreFeed(after: $after, limit: $limit) {
      items {
        id type score
        post {
          id content likeCount replyCount isLiked viewerEmotion createdAt
          reactionCounts { emotion count }
          author { id username displayName }
        }
        article {
          id title slug publishedAt
          author { username displayName }
          board { name owner { username } }
        }
      }
      nextCursor
      hasMore
    }
  }
`;

interface FeedListProps {
  initialItems: FeedItem[];
  initialCursor: string | null;
  initialHasMore: boolean;
}

export function FeedList({
  initialItems,
  initialCursor,
  initialHasMore,
}: FeedListProps) {
  const [items, setItems] = useState<FeedItem[]>(initialItems);
  const [cursor, setCursor] = useState<string | null>(initialCursor);
  const [hasMore, setHasMore] = useState(initialHasMore);
  const [loading, setLoading] = useState(false);

  async function loadMore() {
    if (loading || !hasMore) return;
    setLoading(true);
    try {
      const data = await gqlClient<{ exploreFeed: FeedConnection }>(
        EXPLORE_FEED_QUERY,
        { after: cursor, limit: 20 }
      );
      setItems((prev) => [...prev, ...data.exploreFeed.items]);
      setCursor(data.exploreFeed.nextCursor);
      setHasMore(data.exploreFeed.hasMore);
    } catch (err) {
      console.error("Failed to load more:", err);
    } finally {
      setLoading(false);
    }
  }

  if (items.length === 0) {
    return (
      <div className="text-center py-16 text-gray-400">
        <p>Nothing here yet.</p>
        <p className="text-sm mt-1">Be the first to post!</p>
      </div>
    );
  }

  return (
    <div>
      {items.map((item) => {
        if (item.type === "post" && item.post) {
          return <PostCard key={item.id} post={item.post} />;
        }
        if (item.type === "article" && item.article) {
          return <ArticleCard key={item.id} article={item.article} />;
        }
        return null;
      })}

      {hasMore && (
        <div className="text-center py-6">
          <button
            onClick={loadMore}
            disabled={loading}
            className="text-sm text-gray-500 hover:text-gray-800 disabled:opacity-50"
          >
            {loading ? "Loading…" : "Load more"}
          </button>
        </div>
      )}
    </div>
  );
}
