"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { ArticleCommentForm } from "./article-comment-form";

export interface Comment {
  id: string;
  content: string;
  createdAt: string;
  parentId: string | null;
  author: { username: string; displayName: string | null };
}

function prettyDate(dateInput: string): string {
  const d = new Date(dateInput);
  if (Number.isNaN(d.getTime())) return dateInput;
  return d.toLocaleString("zh-TW", {
    year: "numeric",
    month: "long",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

interface CommentCardProps {
  comment: Comment;
  replies: Comment[];
  articleId: string;
  isReply?: boolean;
}

function CommentCard({ comment, replies, articleId, isReply }: CommentCardProps) {
  const router = useRouter();
  const [showReplyForm, setShowReplyForm] = useState(false);
  const [showReplies, setShowReplies] = useState(replies.length > 0 && replies.length <= 3);
  const displayName = comment.author.displayName ?? comment.author.username;

  function handleReplySuccess() {
    setShowReplyForm(false);
    setShowReplies(true);
    router.refresh();
  }

  return (
    <div className={isReply ? "border-l-2 border-[#3a3f48] pl-4" : ""}>
      <div className={`rounded-xl ${isReply ? "bg-[#252830]" : "bg-[#2d3038]"} p-4`}>
        {/* Header */}
        <div className="mb-2 flex items-center gap-2 text-sm text-[#9ca3b0]">
          <Link
            href={`/@${comment.author.username}`}
            className="font-medium text-[#dce2ec] hover:text-white"
          >
            {displayName}
          </Link>
          <span>·</span>
          <span>{prettyDate(comment.createdAt)}</span>
        </div>

        {/* Content */}
        <p className="whitespace-pre-wrap text-sm text-[#e8ecf4]">{comment.content}</p>

        {/* Actions */}
        <div className="mt-2 flex items-center gap-3">
          <button
            type="button"
            onClick={() => setShowReplyForm((v) => !v)}
            className="text-xs text-[#7a8299] hover:text-[#b0b8cc]"
          >
            {showReplyForm ? "取消" : "回覆"}
          </button>
          {!isReply && replies.length > 0 && (
            <button
              type="button"
              onClick={() => setShowReplies((v) => !v)}
              className="text-xs text-[#7a8299] hover:text-[#b0b8cc]"
            >
              {showReplies
                ? `收起 ${replies.length} 則回覆`
                : `查看 ${replies.length} 則回覆`}
            </button>
          )}
        </div>

        {/* Inline reply form */}
        {showReplyForm && (
          <ArticleCommentForm
            articleId={articleId}
            parentCommentId={comment.id}
            placeholder={`回覆 ${displayName}...`}
            onSuccess={handleReplySuccess}
          />
        )}
      </div>

      {/* Nested replies (shown below the card) */}
      {!isReply && showReplies && replies.length > 0 && (
        <div className="mt-2 space-y-2 pl-4">
          {replies.map((reply) => (
            <CommentCard
              key={reply.id}
              comment={reply}
              replies={[]}
              articleId={articleId}
              isReply
            />
          ))}
        </div>
      )}
    </div>
  );
}

interface CommentThreadProps {
  comments: Comment[];
  articleId: string;
}

export function CommentThread({ comments, articleId }: CommentThreadProps) {
  // Organize into top-level + replies map
  const topLevel = comments.filter((c) => !c.parentId);
  const repliesMap = new Map<string, Comment[]>();
  for (const c of comments) {
    if (c.parentId) {
      const existing = repliesMap.get(c.parentId) ?? [];
      existing.push(c);
      repliesMap.set(c.parentId, existing);
    }
  }

  if (topLevel.length === 0) {
    return (
      <div className="rounded-xl border border-[#2a2e38] bg-[#1c1f27] p-5 text-sm text-[#8a909f]">
        目前尚無回覆。
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {topLevel.map((c) => (
        <CommentCard
          key={c.id}
          comment={c}
          replies={repliesMap.get(c.id) ?? []}
          articleId={articleId}
        />
      ))}
    </div>
  );
}
