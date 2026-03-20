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
  if (mins < 1) return "剛剛";
  if (mins < 60) return `${mins}m`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d`;
  return new Date(isoString).toLocaleDateString("zh-TW");
}

export function PostCard({ post }: { post: Post }) {
  const displayName = post.author.displayName ?? post.author.username;

  return (
    <article className="py-4 border-b border-[var(--app-divider)] last:border-0">
      <div className="flex items-start gap-3">
        {/* Avatar initial */}
        <div className="flex-shrink-0 flex h-8 w-8 items-center justify-center rounded-full bg-[var(--app-accent-bg)] text-xs font-semibold text-[var(--app-accent)]">
          {displayName.slice(0, 1).toUpperCase()}
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1.5">
            <Link
              href={`/@${post.author.username}`}
              className="font-semibold text-sm text-[var(--app-text-heading)] hover:text-[var(--app-accent)] transition-colors"
            >
              {displayName}
            </Link>
            <span className="text-[var(--app-text-muted)] text-xs">
              @{post.author.username}
            </span>
            <span className="text-[var(--app-text-dim)] text-xs">·</span>
            <span className="text-[var(--app-text-muted)] text-xs" title={post.createdAt}>
              {timeAgo(post.createdAt)}
            </span>
          </div>
          <p className="text-sm text-[var(--app-text)] leading-relaxed whitespace-pre-wrap break-words">
            {post.content}
          </p>
          <div className="flex items-center gap-4 mt-2.5">
            <LikeButton
              postId={post.id}
              initialViewerEmotion={post.viewerEmotion}
              initialReactionCounts={post.reactionCounts}
            />
            <Link
              href={`/posts/${post.id}`}
              className="text-xs text-[var(--app-text-muted)] hover:text-[var(--app-accent)] transition-colors"
            >
              {post.replyCount} 則回覆
            </Link>
          </div>
        </div>
      </div>
    </article>
  );
}
