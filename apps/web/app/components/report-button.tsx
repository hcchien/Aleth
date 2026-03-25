"use client";

import { useRef, useState } from "react";
import { useAuth } from "@/lib/auth-context";
import { gqlClient } from "@/lib/gql-client";

const REPORT_POST_MUTATION = `
  mutation ReportPost($postId: ID!, $reason: String!, $note: String) {
    reportPost(postId: $postId, reason: $reason, note: $note)
  }
`;

const REASONS = [
  { value: "spam",            label: "Spam" },
  { value: "harassment",      label: "Harassment" },
  { value: "hate_speech",     label: "Hate speech" },
  { value: "misinformation",  label: "Misinformation" },
  { value: "illegal_content", label: "Illegal content" },
  { value: "other",           label: "Other" },
] as const;

export function ReportButton({ postId, authorId }: { postId: string; authorId: string }) {
  const { user } = useAuth();
  const [open, setOpen] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  // Don't show the report button to the post author or when not logged in.
  if (!user || user.id === authorId) return null;

  async function handleReport(reason: string) {
    setSubmitting(true);
    try {
      await gqlClient<{ reportPost: boolean }>(REPORT_POST_MUTATION, { postId, reason });
      setSubmitted(true);
    } catch {
      // silently fail — report failures shouldn't disrupt reading
    } finally {
      setSubmitting(false);
      setOpen(false);
    }
  }

  if (submitted) {
    return (
      <span className="text-xs text-[var(--app-text-muted)]">Reported</span>
    );
  }

  return (
    <div className="relative" ref={menuRef}>
      <button
        type="button"
        disabled={submitting}
        onClick={() => setOpen((v) => !v)}
        className="text-xs text-[var(--app-text-dim)] hover:text-[var(--app-text-muted)] transition-colors disabled:opacity-50"
        aria-label="Report this post"
        title="Report"
      >
        ⚑
      </button>

      {open && (
        <>
          {/* Backdrop to close on outside click */}
          <div
            className="fixed inset-0 z-30"
            onClick={() => setOpen(false)}
          />
          <div className="absolute right-0 top-5 z-40 min-w-[160px] rounded-lg border border-[var(--app-border-2)] bg-[var(--app-surface)] shadow-lg py-1">
            <p className="px-3 py-1.5 text-xs font-medium text-[var(--app-text-muted)] border-b border-[var(--app-border)]">
              Report reason
            </p>
            {REASONS.map(({ value, label }) => (
              <button
                key={value}
                type="button"
                onClick={() => void handleReport(value)}
                className="block w-full px-3 py-2 text-left text-xs text-[var(--app-text)] hover:bg-[var(--app-surface-2)] transition-colors"
              >
                {label}
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
