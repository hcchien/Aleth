import Link from "next/link";
import { LikeButton } from "./like-button";

interface Post {
  id: string;
  content: string;
  likeCount: number;
  replyCount: number;
  isLiked: boolean;
  viewerEmotion?: string | null;
  reactionCounts: { emotion: string; count: number }[];
  createdAt: string;
  author: {
    id: string;
    username: string;
    displayName: string | null;
  };
}

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

export function PostCard({ post }: { post: Post }) {
  return (
    <article className="py-4 border-b border-gray-100 last:border-0">
      <div className="flex items-start gap-3">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1">
            <Link
              href={`/@${post.author.username}`}
              className="font-medium text-sm hover:underline"
            >
              {post.author.displayName ?? post.author.username}
            </Link>
            <span className="text-gray-400 text-xs">
              @{post.author.username}
            </span>
            <span className="text-gray-300 text-xs">·</span>
            <span className="text-gray-400 text-xs" title={post.createdAt}>
              {timeAgo(post.createdAt)}
            </span>
          </div>
          <p className="text-sm text-gray-800 whitespace-pre-wrap break-words">
            {post.content}
          </p>
          <div className="flex items-center gap-4 mt-2">
            <LikeButton
              postId={post.id}
              initialViewerEmotion={post.viewerEmotion}
              initialReactionCounts={post.reactionCounts}
            />
            <span className="text-xs text-gray-400">
              {post.replyCount} {post.replyCount === 1 ? "reply" : "replies"}
            </span>
          </div>
        </div>
      </div>
    </article>
  );
}
