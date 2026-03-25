"use client";

// PageComposeBox — shown on /p/[slug] for logged-in page members (admin + editor).
// Calls createPagePost mutation and prepends the new post to the feed via onPosted.

import { useCallback, useEffect, useRef, useState } from "react";
import { useAuth } from "@/lib/auth-context";
import { gqlClient } from "@/lib/gql-client";

// ─── GraphQL ──────────────────────────────────────────────────────────────────

const CHECK_MEMBER_QUERY = `
  query CheckPageMember($pageId: ID!) {
    pageMembers(pageId: $pageId) {
      items { role user { id } }
    }
  }
`;

const CREATE_PAGE_POST_MUTATION = `
  mutation CreatePagePost($pageId: ID!, $content: String!) {
    createPagePost(pageId: $pageId, content: $content) {
      id
      content
      createdAt
      replyCount
      viewerEmotion
      reactionCounts { emotion count }
      author { id username displayName }
    }
  }
`;

// ─── Types ────────────────────────────────────────────────────────────────────

export interface PagePost {
  id: string;
  content: string;
  createdAt: string;
  replyCount: number;
  viewerEmotion: string | null;
  reactionCounts: { emotion: string; count: number }[];
  author: { id: string; username: string; displayName: string | null };
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

const MAX_CHARS = 500;

function CharRing({ chars, max }: { chars: number; max: number }) {
  const r = 9;
  const circ = 2 * Math.PI * r;
  const fill = circ * Math.min(1, chars / max);
  const remaining = max - chars;
  const danger = remaining <= 50;
  const color = remaining <= 0 ? "#ef4444" : danger ? "#f59e0b" : "var(--app-accent)";

  return (
    <svg width="24" height="24" viewBox="0 0 24 24" className="rotate-[-90deg]">
      <circle cx="12" cy="12" r={r} fill="none" stroke="var(--app-border-inner)" strokeWidth="2.5" />
      <circle
        cx="12" cy="12" r={r} fill="none"
        stroke={color} strokeWidth="2.5"
        strokeDasharray={`${fill} ${circ}`}
        strokeLinecap="round"
        className="transition-all duration-200"
      />
    </svg>
  );
}

// ─── Component ────────────────────────────────────────────────────────────────

export function PageComposeBox({
  pageId,
  pageSlug,
  onPosted,
}: {
  pageId: string;
  pageSlug: string;
  onPosted: (post: PagePost) => void;
}) {
  const { user, loading: authLoading } = useAuth();
  const [isMember, setIsMember] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const [content, setContent] = useState("");
  const [posting, setPosting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // ── Check membership ──────────────────────────────────────────────────────
  useEffect(() => {
    if (!user || authLoading) return;
    gqlClient<{ pageMembers: { items: { role: string; user: { id: string } }[] } }>(
      CHECK_MEMBER_QUERY,
      { pageId }
    )
      .then((data) => {
        const member = data.pageMembers.items.find((e) => e.user.id === user.id);
        setIsMember(!!member && (member.role === "admin" || member.role === "editor"));
      })
      .catch(() => {});
  }, [user, authLoading, pageId]);

  const handleExpand = useCallback(() => {
    setExpanded(true);
    setTimeout(() => textareaRef.current?.focus(), 50);
  }, []);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
        void handleSubmit();
      }
      if (e.key === "Escape") {
        setExpanded(false);
        setContent("");
        setError(null);
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [content]
  );

  async function handleSubmit() {
    const trimmed = content.trim();
    if (!trimmed || trimmed.length > MAX_CHARS) return;
    setPosting(true);
    setError(null);
    try {
      const data = await gqlClient<{ createPagePost: PagePost }>(
        CREATE_PAGE_POST_MUTATION,
        { pageId, content: trimmed }
      );
      onPosted(data.createPagePost);
      setContent("");
      setExpanded(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to post");
    } finally {
      setPosting(false);
    }
  }

  if (authLoading || !user || !isMember) return null;

  const remaining = MAX_CHARS - content.length;
  const canSubmit = content.trim().length > 0 && remaining >= 0 && !posting;

  return (
    <div className="mb-5 rounded-xl border border-[var(--app-accent-border)] bg-[var(--app-surface-2)] transition-all">
      {!expanded ? (
        <button
          type="button"
          onClick={handleExpand}
          className="flex w-full items-center gap-3 px-4 py-3 text-left"
        >
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-[var(--app-accent-bg)] text-sm font-semibold text-[var(--app-accent)]">
            {(user.displayName ?? user.username)[0].toUpperCase()}
          </div>
          <span className="flex-1 text-sm text-[var(--app-text-dim)]">
            Write something for <strong className="text-[var(--app-accent)]">/p/{pageSlug}</strong>…
          </span>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="shrink-0 text-[var(--app-accent)]">
            <path d="M12 5v14M5 12h14" />
          </svg>
        </button>
      ) : (
        <div className="p-4">
          <textarea
            ref={textareaRef}
            value={content}
            onChange={(e) => setContent(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={`Share something with followers of /p/${pageSlug}…`}
            rows={4}
            className="w-full resize-none bg-transparent text-sm text-[var(--app-text-heading)] placeholder-[var(--app-text-dim)] focus:outline-none leading-relaxed"
          />
          {error && (
            <p className="mb-2 text-xs text-red-400">{error}</p>
          )}
          <div className="flex items-center justify-between pt-2 border-t border-[var(--app-border-inner)]">
            <button
              type="button"
              onClick={() => { setExpanded(false); setContent(""); setError(null); }}
              className="text-xs text-[var(--app-text-dim)] hover:text-[var(--app-text-muted)] transition-colors"
            >
              Cancel
            </button>
            <div className="flex items-center gap-3">
              {content.length > 0 && (
                <span className={`text-xs tabular-nums ${remaining <= 0 ? "text-red-400" : remaining <= 50 ? "text-amber-400" : "text-[var(--app-text-dim)]"}`}>
                  {remaining}
                </span>
              )}
              <CharRing chars={content.length} max={MAX_CHARS} />
              <button
                type="button"
                onClick={() => void handleSubmit()}
                disabled={!canSubmit}
                className="rounded-lg bg-[var(--app-accent)] px-4 py-1.5 text-sm font-bold text-white transition-opacity hover:opacity-90 disabled:opacity-40"
              >
                {posting ? "Posting…" : "Post"}
              </button>
            </div>
          </div>
          <p className="mt-1.5 text-[10px] text-[var(--app-text-dim)]">
            ⌘ + Return to post · Esc to cancel
          </p>
        </div>
      )}
    </div>
  );
}
