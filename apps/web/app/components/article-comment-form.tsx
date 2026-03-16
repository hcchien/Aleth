"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { gqlClient } from "@/lib/gql-client";
import { useAuth } from "@/lib/auth-context";

const CREATE_ARTICLE_COMMENT = `
  mutation CreateArticleComment($articleId: ID!, $content: String!, $parentCommentId: ID) {
    createArticleComment(articleId: $articleId, content: $content, parentCommentId: $parentCommentId) { id }
  }
`;

interface Props {
  articleId: string;
  parentCommentId?: string;
  placeholder?: string;
  onSuccess?: () => void;
}

export function ArticleCommentForm({ articleId, parentCommentId, placeholder, onSuccess }: Props) {
  const { user } = useAuth();
  const router = useRouter();
  const [content, setContent] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!user) {
    if (parentCommentId) return null; // inline reply — don't show prompt
    return (
      <div className="mt-6 rounded-2xl bg-[#2d2f34] px-6 py-6 text-center text-lg text-[#d8dde6]">
        請先登入以發表回覆
      </div>
    );
  }

  async function submit() {
    const text = content.trim();
    if (!text || pending) return;
    setPending(true);
    setError(null);
    try {
      await gqlClient(CREATE_ARTICLE_COMMENT, {
        articleId,
        content: text,
        ...(parentCommentId ? { parentCommentId } : {}),
      });
      setContent("");
      if (onSuccess) {
        onSuccess();
      } else {
        router.refresh();
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "留言失敗");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className={parentCommentId ? "mt-2" : "mt-6 rounded-2xl bg-[#2d2f34] p-4"}>
      <textarea
        value={content}
        onChange={(e) => setContent(e.target.value)}
        placeholder={placeholder ?? "留言..."}
        rows={parentCommentId ? 2 : 3}
        className="min-h-16 w-full rounded-md border border-[#3a3f48] bg-[#1d212b] p-3 text-sm text-[#e5e8ef] focus:outline-none"
      />
      {error && <p className="mt-1 text-xs text-red-400">{error}</p>}
      <div className="mt-2 flex justify-end">
        <button
          type="button"
          onClick={submit}
          disabled={pending || content.trim() === ""}
          className="rounded-md border border-[#3a3f48] px-4 py-1.5 text-sm text-[#e5e8ef] hover:bg-[#1a1e29] disabled:opacity-60"
        >
          {pending ? "送出中..." : "送出"}
        </button>
      </div>
    </div>
  );
}
