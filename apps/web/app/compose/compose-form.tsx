"use client";

import { useState, FormEvent, useEffect, useRef, useCallback } from "react";
import { useRouter } from "next/navigation";
import { gqlClient } from "@/lib/gql-client";
import { useAuth } from "@/lib/auth-context";

const CREATE_POST_MUTATION = `
  mutation CreatePost($input: CreatePostInput!) {
    createPost(input: $input) {
      id
    }
  }
`;

const MAX_CHARS = 500;

export function ComposeForm() {
  const router = useRouter();
  const { user, loading } = useAuth();
  const [content, setContent] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (!loading && !user) {
      router.replace("/");
    }
  }, [loading, user, router]);

  const resize = useCallback(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${el.scrollHeight}px`;
  }, []);

  useEffect(() => {
    textareaRef.current?.focus();
  }, []);

  function handleChange(e: React.ChangeEvent<HTMLTextAreaElement>) {
    if (e.target.value.length > MAX_CHARS) return;
    setContent(e.target.value);
    resize();
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!content.trim() || submitting) return;
    setError(null);
    setSubmitting(true);
    try {
      await gqlClient(CREATE_POST_MUTATION, { input: { content } });
      router.push("/");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to post");
    } finally {
      setSubmitting(false);
    }
  }

  if (loading || !user) return null;

  const remaining = MAX_CHARS - content.length;
  const pct = content.length / MAX_CHARS;
  const charCls =
    pct >= 0.95
      ? "text-red-500"
      : pct >= 0.8
      ? "text-amber-400"
      : "text-[var(--app-text-dim)]";

  return (
    <div className="mx-auto max-w-xl">
      {/* Header */}
      <div className="mb-8 flex items-center justify-between">
        <h1 className="text-[10px] font-bold uppercase tracking-widest text-[var(--app-text-muted)]">
          New Post
        </h1>
        <button
          type="button"
          onClick={() => router.back()}
          className="text-[11px] font-bold uppercase tracking-widest text-[var(--app-text-muted)] hover:text-[var(--app-text-secondary)] transition-colors"
        >
          Cancel
        </button>
      </div>

      <form onSubmit={handleSubmit}>
        {/* Compose area */}
        <div className="rounded-2xl border border-[var(--app-border-inner)] bg-[var(--app-surface-3)] focus-within:border-[var(--app-accent-border)] transition-colors">
          <textarea
            ref={textareaRef}
            value={content}
            onChange={handleChange}
            placeholder="Share something with the network…"
            rows={6}
            className="w-full resize-none bg-transparent px-5 pt-5 pb-4 text-base leading-relaxed text-[var(--app-text)] placeholder:text-[var(--app-text-muted)] outline-none rounded-2xl"
            style={{ minHeight: "180px" }}
            aria-label="Post content"
          />

          {/* Toolbar */}
          <div className="flex items-center justify-between border-t border-[var(--app-border-inner)] px-5 py-3">
            <div className="flex items-center gap-3">
              {content.length > 0 && (
                <>
                  <span className={`text-[11px] font-mono tabular-nums ${charCls}`}>
                    {remaining}
                  </span>
                  <svg width="20" height="20" viewBox="0 0 20 20" className="rotate-[-90deg]" aria-hidden="true">
                    <circle
                      cx="10" cy="10" r="8"
                      fill="none"
                      stroke="var(--app-border-inner)"
                      strokeWidth="2"
                    />
                    <circle
                      cx="10" cy="10" r="8"
                      fill="none"
                      stroke={pct >= 0.95 ? "#ef4444" : pct >= 0.8 ? "#f59e0b" : "var(--app-accent)"}
                      strokeWidth="2"
                      strokeDasharray={`${2 * Math.PI * 8}`}
                      strokeDashoffset={`${2 * Math.PI * 8 * (1 - pct)}`}
                      strokeLinecap="round"
                    />
                  </svg>
                </>
              )}
            </div>

            <button
              type="submit"
              disabled={submitting || !content.trim()}
              className="px-5 py-2 rounded-xl bg-[var(--app-accent)] text-white text-sm font-bold tracking-wide disabled:opacity-40 disabled:cursor-not-allowed hover:opacity-90 transition-opacity"
            >
              {submitting ? "Posting…" : "Publish"}
            </button>
          </div>
        </div>

        {error && (
          <p className="mt-3 text-sm text-red-500">{error}</p>
        )}

        <p className="mt-3 text-[10px] text-[var(--app-text-dim)]">
          ⌘ Return to publish · Esc to go back
        </p>
      </form>
    </div>
  );
}
