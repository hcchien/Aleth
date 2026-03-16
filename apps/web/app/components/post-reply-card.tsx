"use client";

import { useState, useRef } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { gqlClient } from "@/lib/gql-client";
import { useAuth } from "@/lib/auth-context";
import { SignatureBadge, type SignatureInfo } from "./signature-badge";
import { PostReplyForm } from "./post-reply-form";

const REACT_POST = `mutation ReactPost($postId: ID!, $emotion: String!) { reactPost(postId: $postId, emotion: $emotion) }`;
const UNREACT_POST = `mutation UnreactPost($postId: ID!) { unreactPost(postId: $postId) }`;

const EMOTIONS = ["like", "love", "haha", "wow", "sad", "angry"] as const;
type Emotion = (typeof EMOTIONS)[number];

const EMOJI: Record<Emotion, string> = {
  like: "👍", love: "❤️", haha: "😂", wow: "😮", sad: "😢", angry: "😡",
};
const LABEL: Record<Emotion, string> = {
  like: "讚", love: "大心", haha: "哈哈", wow: "哇", sad: "傷心", angry: "生氣",
};

const AVATAR_COLORS = [
  "bg-[#1e3a5f] text-[#7eb8f7]",
  "bg-[#1a3d2e] text-[#6ed4a0]",
  "bg-[#3d2a1a] text-[#f4a84a]",
  "bg-[#2d1a3d] text-[#c084f5]",
  "bg-[#1a2d3d] text-[#67c1d8]",
  "bg-[#3d1a1a] text-[#f48484]",
];

function avatarColor(username: string): string {
  let hash = 0;
  for (let i = 0; i < username.length; i++) hash = username.charCodeAt(i) + ((hash << 5) - hash);
  return AVATAR_COLORS[Math.abs(hash) % AVATAR_COLORS.length];
}

interface PostAuthor {
  id: string;
  username: string;
  displayName: string | null;
  trustLevel: number;
}

export interface PostReplyData {
  id: string;
  content: string;
  replyCount: number;
  likeCount: number;
  viewerEmotion?: string | null;
  reactionCounts: { emotion: string; count: number }[];
  createdAt: string;
  signatureInfo: SignatureInfo;
  author: PostAuthor;
}

function formatRelative(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "剛剛";
  if (mins < 60) return `${mins} 分鐘`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours} 小時`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days} 天`;
  return new Date(iso).toLocaleDateString("zh-TW");
}

export function PostReplyCard({ reply }: { reply: PostReplyData }) {
  const { user } = useAuth();
  const router = useRouter();
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const [showReplyForm, setShowReplyForm] = useState(false);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [pending, setPending] = useState(false);
  const [viewerEmotion, setViewerEmotion] = useState<string | null>(reply.viewerEmotion ?? null);
  const [counts, setCounts] = useState<Record<string, number>>(() => {
    const out: Record<string, number> = {};
    for (const e of EMOTIONS) out[e] = 0;
    for (const r of reply.reactionCounts) out[r.emotion] = r.count;
    return out;
  });

  const totalCount = EMOTIONS.reduce((sum, e) => sum + (counts[e] || 0), 0);
  const topEmotions = EMOTIONS.filter((e) => counts[e] > 0)
    .sort((a, b) => counts[b] - counts[a])
    .slice(0, 3);

  const name = reply.author.displayName ?? reply.author.username;
  const avatarCls = avatarColor(reply.author.username);
  const activeEmotion = viewerEmotion as Emotion | null;

  function openPicker() {
    if (timerRef.current) clearTimeout(timerRef.current);
    setPickerOpen(true);
  }
  function closePicker() {
    timerRef.current = setTimeout(() => setPickerOpen(false), 120);
  }

  function adjust(oldE: string | null, newE: string | null) {
    setCounts((prev) => {
      const next = { ...prev };
      if (oldE) next[oldE] = Math.max(0, (next[oldE] || 0) - 1);
      if (newE) next[newE] = (next[newE] || 0) + 1;
      return next;
    });
  }

  async function react(emotion: Emotion) {
    if (!user || pending) return;
    setPending(true);
    setPickerOpen(false);
    const oldE = viewerEmotion;
    const nextE = oldE === emotion ? null : emotion;
    setViewerEmotion(nextE);
    adjust(oldE, nextE);
    try {
      if (nextE === null) await gqlClient(UNREACT_POST, { postId: reply.id });
      else await gqlClient(REACT_POST, { postId: reply.id, emotion: nextE });
    } catch {
      setViewerEmotion(oldE);
      adjust(nextE, oldE);
    } finally {
      setPending(false);
    }
  }

  function handleReplySuccess() {
    setShowReplyForm(false);
    router.refresh();
  }

  return (
    <div className="flex gap-2.5">
      {/* Avatar */}
      <Link href={`/@${reply.author.username}`} className="shrink-0 mt-0.5">
        <span className={`flex h-8 w-8 items-center justify-center rounded-full text-sm font-semibold ${avatarCls}`}>
          {name.slice(0, 1).toUpperCase()}
        </span>
      </Link>

      <div className="flex-1 min-w-0">
        {/* Comment bubble */}
        <div className="relative inline-block max-w-full">
          {/* Emoji picker popup */}
          {pickerOpen && user && (
            <div
              className="absolute -top-12 left-0 z-30 flex items-center gap-0.5 rounded-full border border-[#3a3f4e] bg-[#1a1f2e] px-3 py-2 shadow-2xl"
              onMouseEnter={openPicker}
              onMouseLeave={closePicker}
            >
              {EMOTIONS.map((emotion) => (
                <button
                  key={emotion}
                  type="button"
                  onClick={() => react(emotion)}
                  title={LABEL[emotion]}
                  className={`rounded-full p-1 text-2xl transition-transform duration-100 hover:scale-125 active:scale-110 ${
                    viewerEmotion === emotion ? "scale-110 brightness-125" : ""
                  }`}
                >
                  {EMOJI[emotion]}
                </button>
              ))}
            </div>
          )}

          <div className="rounded-2xl bg-[#1e2330] px-4 py-2.5">
            <div className="flex items-baseline gap-2">
              <Link
                href={`/@${reply.author.username}`}
                className="text-sm font-semibold text-[#f0f2f6] hover:text-white"
              >
                {name}
              </Link>
              <SignatureBadge info={reply.signatureInfo} />
            </div>
            <p className="mt-0.5 text-sm leading-relaxed text-[#d5d9e2] whitespace-pre-wrap">
              {reply.content}
            </p>
          </div>

          {/* Reaction bubble — overlaid on bottom-right of bubble */}
          {totalCount > 0 && (
            <div className="absolute -bottom-3 right-2 flex items-center gap-0.5 rounded-full border border-[#0b0d12] bg-[#252b3b] px-1.5 py-0.5 text-xs text-[#9ea4b0] shadow">
              <span className="flex -space-x-0.5">
                {topEmotions.map((e) => (
                  <span key={e} className="text-[11px]">{EMOJI[e]}</span>
                ))}
              </span>
              <span>{totalCount}</span>
            </div>
          )}
        </div>

        {/* Action row */}
        <div className={`flex items-center gap-3 px-2 text-xs text-[#6b7280] ${totalCount > 0 ? "mt-4" : "mt-1.5"}`}>
          <span>{formatRelative(reply.createdAt)}</span>
          <button
            type="button"
            disabled={!user || pending}
            onMouseEnter={() => user && openPicker()}
            onMouseLeave={closePicker}
            onClick={() => activeEmotion ? react(activeEmotion) : react("like")}
            className={`font-semibold transition-colors disabled:opacity-40 ${
              activeEmotion ? "text-blue-400" : "hover:text-[#d0d5e2]"
            }`}
          >
            {activeEmotion ? LABEL[activeEmotion] : "讚"}
          </button>
          <button
            type="button"
            onClick={() => setShowReplyForm((v) => !v)}
            className="font-semibold hover:text-[#d0d5e2] transition-colors"
          >
            回覆
          </button>
          {reply.replyCount > 0 && (
            <Link
              href={`/posts/${reply.id}`}
              className="hover:text-[#d0d5e2] transition-colors"
            >
              {reply.replyCount} 則回覆
            </Link>
          )}
        </div>

        {/* Reply form */}
        {showReplyForm && (
          <div className="mt-3">
            <PostReplyForm postId={reply.id} compact onSuccess={handleReplySuccess} />
          </div>
        )}
      </div>
    </div>
  );
}
