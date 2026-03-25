"use client";

import { useState, useRef, useEffect } from "react";
import Link from "next/link";
import { gqlClient } from "@/lib/gql-client";
import { useAuth } from "@/lib/auth-context";

const REACT_POST = `mutation ReactPost($postId: ID!, $emotion: String!) { reactPost(postId: $postId, emotion: $emotion) }`;
const UNREACT_POST = `mutation UnreactPost($postId: ID!) { unreactPost(postId: $postId) }`;
const RESHARE_POST = `mutation ResharePost($postId: ID!, $input: ResharePostInput!) { resharePost(postId: $postId, input: $input) { id } }`;
const FRIEND_REACTORS_QUERY = `query FriendReactors($postId: ID!) {
  postFriendReactors(postId: $postId, limit: 5) { id username displayName emotion }
}`;

// ─── FriendReactorLine ────────────────────────────────────────────────────────

interface FriendReactorLineProps {
  reactors: { id: string; username: string; displayName: string | null; emotion: string }[];
  totalCount: number;
}

function FriendReactorLine({ reactors, totalCount }: FriendReactorLineProps) {
  if (reactors.length === 0) return null;

  const shown = reactors.slice(0, 2);
  const others = totalCount - reactors.length; // non-friend reactors

  const parts: React.ReactNode[] = [];
  shown.forEach((r, i) => {
    parts.push(
      <span key={r.id} className="font-medium text-[#9ea4b0]">
        {r.displayName ?? r.username}
      </span>,
      <span key={`e-${r.id}`}>&nbsp;{EMOJI[r.emotion as Emotion] ?? "👍"}</span>
    );
    if (i < shown.length - 1) parts.push(<span key={`sep-${i}`}> · </span>);
  });

  if (others > 0) {
    parts.push(
      <span key="others"> 和其他 {others} 人</span>
    );
  }

  return <span className="leading-relaxed">{parts} 對此有反應</span>;
}

// ─── EMOTIONS ─────────────────────────────────────────────────────────────────

const EMOTIONS = ["like", "love", "haha", "wow", "sad", "angry"] as const;
type Emotion = (typeof EMOTIONS)[number];

const EMOJI: Record<Emotion, string> = {
  like: "👍",
  love: "❤️",
  haha: "😂",
  wow: "😮",
  sad: "😢",
  angry: "😡",
};

const LABEL: Record<Emotion, string> = {
  like: "讚",
  love: "大心",
  haha: "哈哈",
  wow: "哇",
  sad: "傷心",
  angry: "生氣",
};

// ─── Icons ────────────────────────────────────────────────────────────────────

function IconChatBubble() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z" />
    </svg>
  );
}

function IconToken({ filled }: { filled: boolean }) {
  // Hexagonal token / vouch icon
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill={filled ? "currentColor" : "none"} stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <polygon points="12 2 19 6 19 18 12 22 5 18 5 6" />
      {filled && <polygon points="12 2 19 6 19 18 12 22 5 18 5 6" fill="currentColor" stroke="none" />}
      <path d="M9 12l2 2 4-4" strokeWidth="1.8" />
    </svg>
  );
}

function IconShare() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M4 12v8a2 2 0 002 2h12a2 2 0 002-2v-8M16 6l-4-4-4 4M12 2v13" />
    </svg>
  );
}

interface ReactBarProps {
  postId: string;
  initialViewerEmotion?: string | null;
  initialReactionCounts: { emotion: string; count: number }[];
  replyCount: number;
  replyHref?: string;
  onReply?: () => void;
  shareHref?: string;
  postPreview?: { content: string; authorName: string };
}

export function ReactBar({
  postId,
  initialViewerEmotion,
  initialReactionCounts,
  replyCount,
  replyHref,
  onReply,
  shareHref,
  postPreview,
}: ReactBarProps) {
  const { user } = useAuth();
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const [viewerEmotion, setViewerEmotion] = useState<string | null>(
    initialViewerEmotion ?? null
  );
  const [counts, setCounts] = useState<Record<string, number>>(() => {
    const out: Record<string, number> = {};
    for (const e of EMOTIONS) out[e] = 0;
    for (const r of initialReactionCounts) out[r.emotion] = r.count;
    return out;
  });
  const [pending, setPending] = useState(false);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [showShare, setShowShare] = useState(false);

  // Progressive loading: friends who reacted. Fetched lazily after mount so
  // it never blocks the initial feed render.
  type FriendReactor = { id: string; username: string; displayName: string | null; emotion: string };
  const [friendReactors, setFriendReactors] = useState<FriendReactor[] | null>(null);

  useEffect(() => {
    if (!user) return; // unauthenticated — skip
    let cancelled = false;
    // Small delay so the fetch doesn't race with the initial paint.
    const timer = setTimeout(async () => {
      try {
        const data = await gqlClient<{ postFriendReactors: FriendReactor[] }>(
          FRIEND_REACTORS_QUERY,
          { postId: postId }
        );
        if (!cancelled) setFriendReactors(data.postFriendReactors ?? []);
      } catch {
        // Non-critical — silently ignore errors.
      }
    }, 300);
    return () => { cancelled = true; clearTimeout(timer); };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [postId, user?.id]);
  const [copied, setCopied] = useState(false);
  const [reshareView, setReshareView] = useState(false);
  const [reshareContent, setReshareContent] = useState("");
  const [reshareSubmitting, setReshareSubmitting] = useState(false);
  const [reshared, setReshared] = useState(false);

  function getShareUrl() {
    const path = shareHref ?? replyHref ?? "";
    return path ? `${window.location.origin}${path}` : window.location.href;
  }

  async function copyLink() {
    try {
      await navigator.clipboard.writeText(getShareUrl());
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // fallback: select the input
    }
  }

  async function submitReshare() {
    if (!user || reshareSubmitting) return;
    setReshareSubmitting(true);
    try {
      await gqlClient(RESHARE_POST, { postId, input: { content: reshareContent.trim() || null } });
      setReshared(true);
      setTimeout(() => {
        setShowShare(false);
        setReshareView(false);
        setReshared(false);
        setReshareContent("");
      }, 1200);
    } catch {
      // ignore
    } finally {
      setReshareSubmitting(false);
    }
  }

  const totalCount = EMOTIONS.reduce((sum, e) => sum + (counts[e] || 0), 0);
  const topEmotions = EMOTIONS.filter((e) => counts[e] > 0)
    .sort((a, b) => counts[b] - counts[a])
    .slice(0, 3);

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
      if (nextE === null) await gqlClient(UNREACT_POST, { postId });
      else await gqlClient(REACT_POST, { postId, emotion: nextE });
    } catch {
      setViewerEmotion(oldE);
      adjust(nextE, oldE);
    } finally {
      setPending(false);
    }
  }

  const activeEmotion = viewerEmotion as Emotion | null;
  const btnLabel = activeEmotion ? LABEL[activeEmotion] : "讚";
  const btnEmoji = activeEmotion ? EMOJI[activeEmotion] : "👍";

  return (
    <div>
      {/* Friend reactor line — progressively loaded */}
      {friendReactors && friendReactors.length > 0 && (
        <div className="mb-3 text-xs text-[var(--app-text-muted)]">
          <FriendReactorLine reactors={friendReactors} totalCount={totalCount} />
        </div>
      )}

      {/* Action bar — Stitch style: icon + label, no dividers */}
      <div className="relative flex items-center gap-6">
        {/* Emoji picker popup */}
        {pickerOpen && user && (
          <div
            className="absolute bottom-full left-0 z-30 mb-2 flex items-center gap-0.5 rounded-full border border-[var(--app-border)] bg-[var(--app-surface-2)] px-3 py-2 shadow-2xl"
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

        {/* Comments */}
        {onReply ? (
          <button
            type="button"
            onClick={onReply}
            className="flex items-center gap-1.5 text-[var(--app-text-muted)] hover:text-[var(--app-accent)] transition-colors"
          >
            <IconChatBubble />
            <span className="text-[10px] font-bold tracking-wider uppercase">
              {replyCount > 0 ? `${replyCount} Comments` : "Comment"}
            </span>
          </button>
        ) : replyHref ? (
          <Link
            href={replyHref}
            className="flex items-center gap-1.5 text-[var(--app-text-muted)] hover:text-[var(--app-accent)] transition-colors"
          >
            <IconChatBubble />
            <span className="text-[10px] font-bold tracking-wider uppercase">
              {replyCount > 0 ? `${replyCount} Comments` : "Comment"}
            </span>
          </Link>
        ) : (
          <span className="flex items-center gap-1.5 text-[var(--app-text-muted)]">
            <IconChatBubble />
            <span className="text-[10px] font-bold tracking-wider uppercase">
              {replyCount > 0 ? `${replyCount} Comments` : "Comment"}
            </span>
          </span>
        )}

        {/* Vouches (reactions) */}
        <button
          type="button"
          disabled={!user || pending}
          onMouseEnter={() => user && openPicker()}
          onMouseLeave={closePicker}
          onClick={() => user && (activeEmotion ? react(activeEmotion) : react("like"))}
          className={`flex items-center gap-1.5 transition-colors disabled:opacity-40 ${
            activeEmotion
              ? "text-[var(--app-secondary)]"
              : "text-[var(--app-text-muted)] hover:text-[var(--app-secondary)]"
          }`}
        >
          <IconToken filled={!!activeEmotion} />
          <span className="text-[10px] font-bold tracking-wider uppercase">
            {totalCount > 0 ? `${totalCount} Vouches` : "Vouch"}
          </span>
        </button>

        {/* Share — icon only, pushed right */}
        <button
          type="button"
          onClick={() => setShowShare(true)}
          className="flex items-center gap-1.5 text-[var(--app-text-muted)] hover:text-[var(--app-text-heading)] transition-colors ml-auto"
        >
          <IconShare />
        </button>
      </div>

      {/* Share modal */}
      {showShare && (
        <div
          className="fixed inset-0 z-50 flex items-end justify-center sm:items-center"
          onClick={() => { setShowShare(false); setReshareView(false); setReshareContent(""); }}
        >
          <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" />
          <div
            className="relative w-full max-w-sm rounded-t-2xl sm:rounded-2xl border border-[var(--app-border)] bg-[var(--app-surface-2)] shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          >
            {reshareView ? (
              <>
                <div className="flex items-center justify-between border-b border-[var(--app-border-inner)] px-5 py-4">
                  <button type="button" onClick={() => setReshareView(false)} className="text-sm text-[var(--app-text-muted)] hover:text-[var(--app-text-heading)]">
                    ← 返回
                  </button>
                  <h2 className="font-semibold text-[var(--app-text-heading)]">分享到動態</h2>
                  <button type="button" onClick={() => { setShowShare(false); setReshareView(false); setReshareContent(""); }} className="flex h-8 w-8 items-center justify-center rounded-full text-[var(--app-text-muted)] hover:bg-[var(--app-hover)] hover:text-[var(--app-text-heading)] transition-colors">✕</button>
                </div>
                <div className="px-5 py-4 space-y-3">
                  <textarea
                    value={reshareContent}
                    onChange={(e) => setReshareContent(e.target.value)}
                    placeholder="留個話吧…"
                    rows={3}
                    className="w-full resize-none rounded-xl border border-[var(--app-border)] bg-[var(--app-bg)] px-3 py-2.5 text-sm text-[var(--app-text)] placeholder:text-[var(--app-text-dim)] focus:outline-none focus:border-[var(--app-accent)]"
                  />
                  {postPreview && (
                    <div className="rounded-xl border border-[var(--app-border-inner)] bg-[var(--app-bg)] px-4 py-3">
                      <p className="mb-1 text-xs text-[var(--app-text-muted)]">{postPreview.authorName}</p>
                      <p className="text-sm text-[var(--app-text-secondary)] line-clamp-3">
                        {postPreview.content.length > 150 ? postPreview.content.slice(0, 150) + "…" : postPreview.content}
                      </p>
                    </div>
                  )}
                  <button type="button" onClick={submitReshare} disabled={reshareSubmitting || reshared} className="w-full rounded-xl bg-[var(--app-accent)] py-2.5 text-sm font-semibold text-white transition-opacity hover:opacity-90 disabled:opacity-60">
                    {reshared ? "已分享！" : reshareSubmitting ? "分享中…" : "分享"}
                  </button>
                </div>
              </>
            ) : (
              <>
                <div className="flex items-center justify-between border-b border-[var(--app-border-inner)] px-5 py-4">
                  <h2 className="font-semibold text-[var(--app-text-heading)]">分享</h2>
                  <button type="button" onClick={() => setShowShare(false)} className="flex h-8 w-8 items-center justify-center rounded-full text-[var(--app-text-muted)] hover:bg-[var(--app-hover)] hover:text-[var(--app-text-heading)] transition-colors">✕</button>
                </div>
                <div className="px-5 py-5 space-y-4">
                  <div className="flex items-center gap-2 rounded-xl border border-[var(--app-border)] bg-[var(--app-bg)] px-3 py-2.5">
                    <span className="flex-1 truncate text-sm text-[var(--app-text-muted)] font-mono">
                      {typeof window !== "undefined" ? getShareUrl() : ""}
                    </span>
                  </div>
                  <div className="space-y-1">
                    <button type="button" onClick={copyLink} className="flex w-full items-center gap-4 rounded-xl px-3 py-3 text-left hover:bg-[var(--app-hover)] transition-colors">
                      <span className="flex h-10 w-10 items-center justify-center rounded-full bg-[var(--app-surface-3)] text-xl">🔗</span>
                      <div>
                        <div className="text-sm font-medium text-[var(--app-text-heading)]">{copied ? "已複製！" : "複製連結"}</div>
                        <div className="text-xs text-[var(--app-text-muted)]">複製貼文連結</div>
                      </div>
                      {copied && <span className="ml-auto text-sm text-[var(--app-secondary)]">✓</span>}
                    </button>
                    {user && postPreview && (
                      <button type="button" onClick={() => setReshareView(true)} className="flex w-full items-center gap-4 rounded-xl px-3 py-3 text-left hover:bg-[var(--app-hover)] transition-colors">
                        <span className="flex h-10 w-10 items-center justify-center rounded-full bg-[var(--app-surface-3)] text-xl">↗</span>
                        <div>
                          <div className="text-sm font-medium text-[var(--app-text-heading)]">分享到動態</div>
                          <div className="text-xs text-[var(--app-text-muted)]">轉貼到你的個人動態</div>
                        </div>
                      </button>
                    )}
                  </div>
                </div>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
