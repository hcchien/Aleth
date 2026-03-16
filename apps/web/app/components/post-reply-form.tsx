"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { gqlClient } from "@/lib/gql-client";
import { useAuth } from "@/lib/auth-context";

const REPLY_POST_MUTATION = `
  mutation ReplyPost($postId: ID!, $input: CreatePostInput!) {
    replyPost(postId: $postId, input: $input) { id }
  }
`;

interface Props {
  postId: string;
  compact?: boolean;
  onSuccess?: () => void;
}

export function PostReplyForm({ postId, compact, onSuccess }: Props) {
  const { user } = useAuth();
  const router = useRouter();
  const [content, setContent] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!user) {
    if (compact) return null;
    return (
      <div className="mt-6 rounded-2xl bg-[#1e232e] px-6 py-6 text-center text-sm text-[#9ea4b0]">
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
      await gqlClient(REPLY_POST_MUTATION, {
        postId,
        input: { content: text },
      });
      setContent("");
      if (onSuccess) {
        onSuccess();
      } else {
        router.refresh();
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "回覆失敗");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className={compact ? "mt-2" : "mt-6 rounded-2xl border border-[#2a2e38] bg-[#1a1e28] p-4"}>
      <textarea
        value={content}
        onChange={(e) => setContent(e.target.value)}
        placeholder="回覆..."
        rows={compact ? 2 : 3}
        className="w-full rounded-lg border border-[#2a2e38] bg-[#0f1117] p-3 text-sm text-[#e5e8ef] placeholder:text-[#5a6070] focus:border-[#4a5060] focus:outline-none"
      />
      {error && <p className="mt-2 text-xs text-red-400">{error}</p>}
      <div className="mt-2 flex justify-end">
        <button
          type="button"
          onClick={submit}
          disabled={pending || content.trim() === ""}
          className="rounded-lg border border-[#3a3f48] px-4 py-2 text-sm text-[#e5e8ef] hover:bg-[#1a1e29] disabled:opacity-60"
        >
          {pending ? "送出中…" : "送出回覆"}
        </button>
      </div>
    </div>
  );
}
