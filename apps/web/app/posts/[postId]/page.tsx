import { notFound } from "next/navigation";
import Link from "next/link";
import { gql } from "@/lib/gql";
import { ForumShell } from "@/app/components/forum-shell";
import { SignatureBadge, type SignatureInfo } from "@/app/components/signature-badge";
import { TrustBadge } from "@/app/components/trust-badge";
import { ReactBar } from "@/app/components/react-bar";
import { PostReplyForm } from "@/app/components/post-reply-form";
import { PostReplyCard, type PostReplyData } from "@/app/components/post-reply-card";

const POST_QUERY = `
  query Post($id: ID!) {
    post(id: $id) {
      id
      content
      replyCount
      likeCount
      isLiked
      viewerEmotion
      reactionCounts { emotion count }
      createdAt
      parentId
      signatureInfo { isSigned isVerified contentHash signature algorithm explanation }
      author { id username displayName trustLevel }
    }
  }
`;

const POST_REPLIES_QUERY = `
  query PostReplies($postId: ID!, $limit: Int) {
    postReplies(postId: $postId, limit: $limit) {
      id
      content
      replyCount
      likeCount
      viewerEmotion
      reactionCounts { emotion count }
      createdAt
      signatureInfo { isSigned isVerified contentHash signature algorithm explanation }
      author { id username displayName trustLevel }
    }
  }
`;

interface PostAuthor {
  id: string;
  username: string;
  displayName: string | null;
  trustLevel: number;
}

interface PostDetail extends PostReplyData {
  isLiked: boolean;
  parentId?: string | null;
}

interface PageProps {
  params: Promise<{ postId: string }>;
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString("zh-TW", {
    year: "numeric",
    month: "long",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}


function AuthorChip({ author }: { author: PostAuthor }) {
  const name = author.displayName ?? author.username;
  return (
    <div className="flex items-center gap-3">
      <span className="flex h-8 w-8 items-center justify-center rounded-full bg-[#3a2a1a] text-sm text-[#f4ad67]">
        {name.slice(0, 1).toUpperCase()}
      </span>
      <Link
        href={`/@${author.username}`}
        className="font-semibold text-[#e8eaf2] hover:text-white"
      >
        {name}
      </Link>
      <TrustBadge level={author.trustLevel} />
    </div>
  );
}


export default async function PostPage({ params }: PageProps) {
  const { postId } = await params;

  let post: PostDetail | null = null;
  let replies: PostDetail[] = [];
  let fetchFailed = false;

  try {
    const data = await gql<{ post: PostDetail | null }>(
      POST_QUERY,
      { id: postId },
      { revalidate: 0 }
    );
    post = data.post;
  } catch {
    fetchFailed = true;
  }

  if (!fetchFailed && !post) {
    notFound();
  }

  if (fetchFailed || !post) {
    return (
      <ForumShell>
        <Link href="/" className="mb-5 inline-flex items-center gap-2 text-base text-[#d3d8e2] hover:text-white">
          ←　返回列表
        </Link>
        <div className="rounded-xl border border-[#333944] bg-[#0f1117] px-6 py-10 text-center text-sm text-[#aeb4bf]">
          無法載入貼文，請稍後再試。
        </div>
      </ForumShell>
    );
  }

  try {
    const data = await gql<{ postReplies: PostDetail[] }>(
      POST_REPLIES_QUERY,
      { postId, limit: 100 },
      { revalidate: 0 }
    );
    replies = data.postReplies ?? [];
  } catch {
    replies = [];
  }

  const authorName = post.author.displayName ?? post.author.username;

  return (
    <ForumShell>
      <Link
        href="/"
        className="mb-5 inline-flex items-center gap-2 text-base text-[#d3d8e2] hover:text-white"
      >
        ←　返回列表
      </Link>

      {/* Main post */}
      <article className="mb-6 rounded-2xl border border-[#333944] bg-gradient-to-b from-[#1c2030] to-[#151922] p-7">
        <div className="mb-4 flex items-center justify-between">
          <AuthorChip author={post.author} />
          <div className="flex items-center gap-3 text-sm text-[#9ea4b0]">
            <span title={post.createdAt}>{formatDate(post.createdAt)}</span>
            <SignatureBadge info={post.signatureInfo} />
          </div>
        </div>

        <p className="mb-5 whitespace-pre-wrap text-base leading-relaxed text-[#d5d9e2]">
          {post.content}
        </p>

        <ReactBar
          postId={post.id}
          initialViewerEmotion={post.viewerEmotion}
          initialReactionCounts={post.reactionCounts}
          replyCount={post.replyCount}
          replyHref={`/posts/${post.id}`}
          postPreview={{ content: post.content, authorName: authorName }}
        />
      </article>

      {/* Replies */}
      <section>
        <h2 className="mb-4 font-serif text-2xl text-[#e6e7ea]">
          回覆 ({replies.length})
        </h2>
        <div className="space-y-5">
          {replies.map((reply) => (
            <PostReplyCard key={reply.id} reply={reply} />
          ))}
          {replies.length === 0 && (
            <div className="rounded-xl border border-[#2a2e38] bg-[#0f1117] px-6 py-8 text-center text-sm text-[#7a8090]">
              尚無回覆。
            </div>
          )}
        </div>
      </section>

      <PostReplyForm postId={post.id} />
    </ForumShell>
  );
}
