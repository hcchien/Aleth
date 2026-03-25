"use client";

import { useState, useRef, useEffect, useCallback } from "react";
import Image from "next/image";
import { useAuth } from "@/lib/auth-context";
import { gqlClient } from "@/lib/gql-client";
import { getAccessToken } from "@/lib/auth";

const CREATE_POST_MUTATION = `
  mutation CreatePost($input: CreatePostInput!) {
    createPost(input: $input) {
      id content createdAt
      author { id username displayName trustLevel }
    }
  }
`;

const MAX_CHARS = 500;
const MAX_IMAGES = 4;
const DRAFT_KEY = "compose_draft";

// Avatar color — deterministic by username
const AVATAR_COLORS = [
  "bg-blue-500/20 text-blue-400",
  "bg-violet-500/20 text-violet-400",
  "bg-emerald-500/20 text-emerald-400",
  "bg-amber-500/20 text-amber-400",
  "bg-rose-500/20 text-rose-400",
  "bg-cyan-500/20 text-cyan-400",
];
function avatarColor(name: string) {
  return AVATAR_COLORS[name.charCodeAt(0) % AVATAR_COLORS.length];
}

interface ImagePreview {
  localUrl: string;   // blob URL for instant preview
  remoteUrl?: string; // after upload completes
  uploading: boolean;
  error?: string;
}

interface ComposeBoxProps {
  onPosted?: (postId: string) => void;
  placeholder?: string;
}

export function ComposeBox({ onPosted, placeholder = "Share something with the network…" }: ComposeBoxProps) {
  const { user } = useAuth();
  const [expanded, setExpanded] = useState(false);
  const [text, setText] = useState("");
  const [images, setImages] = useState<ImagePreview[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Restore draft on mount
  useEffect(() => {
    const saved = localStorage.getItem(DRAFT_KEY);
    if (saved) {
      setText(saved);
      setExpanded(true);
    }
  }, []);

  // Save draft on change
  useEffect(() => {
    if (text) {
      localStorage.setItem(DRAFT_KEY, text);
    } else {
      localStorage.removeItem(DRAFT_KEY);
    }
  }, [text]);

  // Revoke blob URLs on unmount
  useEffect(() => {
    return () => {
      images.forEach((img) => URL.revokeObjectURL(img.localUrl));
    };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // Auto-resize textarea
  const resize = useCallback(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${el.scrollHeight}px`;
  }, []);

  function handleExpand() {
    setExpanded(true);
    setTimeout(() => {
      textareaRef.current?.focus();
      resize();
    }, 0);
  }

  function handleChange(e: React.ChangeEvent<HTMLTextAreaElement>) {
    if (e.target.value.length > MAX_CHARS) return;
    setText(e.target.value);
    resize();
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLTextAreaElement>) {
    if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
      e.preventDefault();
      void handleSubmit();
    }
    if (e.key === "Escape" && !text && images.length === 0) {
      setExpanded(false);
    }
  }

  async function uploadFile(file: File, index: number) {
    const token = getAccessToken();
    const formData = new FormData();
    formData.append("file", file);
    try {
      const res = await fetch("/api/upload", {
        method: "POST",
        headers: token ? { authorization: `Bearer ${token}` } : {},
        body: formData,
      });
      const json = await res.json() as { url?: string; error?: string };
      if (!res.ok || !json.url) throw new Error(json.error ?? "Upload failed");
      setImages((prev) =>
        prev.map((img, i) => (i === index ? { ...img, remoteUrl: json.url, uploading: false } : img))
      );
    } catch (err) {
      setImages((prev) =>
        prev.map((img, i) =>
          i === index ? { ...img, uploading: false, error: err instanceof Error ? err.message : "Failed" } : img
        )
      );
    }
  }

  function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const files = Array.from(e.target.files ?? []);
    if (!files.length) return;
    e.target.value = ""; // reset so same file can be re-selected

    const slots = MAX_IMAGES - images.length;
    const toAdd = files.slice(0, slots);

    const newPreviews: ImagePreview[] = toAdd.map((file) => ({
      localUrl: URL.createObjectURL(file),
      uploading: true,
    }));

    setImages((prev) => {
      const updated = [...prev, ...newPreviews];
      // kick off uploads
      toAdd.forEach((file, i) => void uploadFile(file, prev.length + i));
      return updated;
    });
    setExpanded(true);
  }

  function removeImage(index: number) {
    setImages((prev) => {
      URL.revokeObjectURL(prev[index].localUrl);
      return prev.filter((_, i) => i !== index);
    });
  }

  async function handleSubmit() {
    const trimmed = text.trim();
    const hasImages = images.some((img) => img.remoteUrl);
    if ((!trimmed && !hasImages) || submitting) return;

    // Wait if any image is still uploading
    const uploading = images.some((img) => img.uploading);
    if (uploading) {
      setError("Images are still uploading…");
      return;
    }

    setSubmitting(true);
    setError(null);
    try {
      const imageUrls = images.filter((img) => img.remoteUrl).map((img) => img.remoteUrl!);
      const data = await gqlClient<{ createPost: { id: string } }>(
        CREATE_POST_MUTATION,
        { input: { content: trimmed || " ", imageUrls } }
      );
      setText("");
      localStorage.removeItem(DRAFT_KEY);
      images.forEach((img) => URL.revokeObjectURL(img.localUrl));
      setImages([]);
      setExpanded(false);
      onPosted?.(data.createPost.id);
    } catch {
      setError("Failed to post. Please try again.");
    } finally {
      setSubmitting(false);
    }
  }

  function handleDiscard() {
    setText("");
    localStorage.removeItem(DRAFT_KEY);
    images.forEach((img) => URL.revokeObjectURL(img.localUrl));
    setImages([]);
    setExpanded(false);
    setError(null);
  }

  const remaining = MAX_CHARS - text.length;
  const pct = text.length / MAX_CHARS;
  const charCls =
    pct >= 0.95 ? "text-red-500" : pct >= 0.8 ? "text-amber-400" : "text-[var(--app-text-dim)]";

  const displayName = user ? (user.displayName ?? user.username) : "";
  const avatarChar = displayName ? displayName[0].toUpperCase() : "?";
  const avatarCls = user
    ? avatarColor(user.username)
    : "bg-[var(--app-surface-4)] text-[var(--app-text-muted)]";

  if (!user) return null;

  const canPost = (text.trim().length > 0 || images.some((img) => img.remoteUrl)) && !submitting;
  const hasContent = text.length > 0 || images.length > 0;

  return (
    <div className="mb-10">
      <div
        className={`rounded-2xl border transition-all ${
          expanded
            ? "border-[var(--app-accent-border)] bg-[var(--app-surface-3)] shadow-sm"
            : "border-[var(--app-border-inner)] bg-[var(--app-surface-3)] hover:border-[var(--app-accent-border)]"
        }`}
      >
        <div className="flex gap-3 p-4">
          {/* Avatar */}
          <div
            className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-xl text-sm font-bold ${avatarCls}`}
          >
            {avatarChar}
          </div>

          {/* Input area */}
          <div className="flex-1 min-w-0">
            {!expanded ? (
              <button
                type="button"
                onClick={handleExpand}
                className="w-full text-left text-sm text-[var(--app-text-muted)] cursor-text py-1.5 leading-relaxed"
              >
                {placeholder}
              </button>
            ) : (
              <textarea
                ref={textareaRef}
                value={text}
                onChange={handleChange}
                onKeyDown={handleKeyDown}
                placeholder={placeholder}
                rows={3}
                className="w-full resize-none bg-transparent text-sm leading-relaxed text-[var(--app-text)] placeholder:text-[var(--app-text-muted)] outline-none"
                style={{ minHeight: "72px" }}
                aria-label="Compose post"
              />
            )}
          </div>
        </div>

        {/* Image previews */}
        {images.length > 0 && (
          <div
            className={`px-4 pb-3 grid gap-2 ${
              images.length === 1 ? "grid-cols-1" : "grid-cols-2"
            }`}
          >
            {images.map((img, i) => (
              <div
                key={img.localUrl}
                className="relative overflow-hidden rounded-xl bg-[var(--app-surface-4)]"
                style={{ aspectRatio: images.length === 1 ? "16/9" : "1" }}
              >
                <Image
                  src={img.localUrl}
                  alt={`Attachment ${i + 1}`}
                  fill
                  className={`object-cover transition-opacity ${img.uploading ? "opacity-60" : "opacity-100"}`}
                  unoptimized
                />
                {/* Uploading spinner */}
                {img.uploading && (
                  <div className="absolute inset-0 flex items-center justify-center">
                    <div className="h-5 w-5 rounded-full border-2 border-white/40 border-t-white animate-spin" />
                  </div>
                )}
                {/* Error badge */}
                {img.error && (
                  <div className="absolute inset-0 flex items-center justify-center bg-black/60">
                    <span className="text-[10px] text-red-400 font-bold px-2 text-center">{img.error}</span>
                  </div>
                )}
                {/* Remove button */}
                <button
                  type="button"
                  onClick={() => removeImage(i)}
                  className="absolute top-1.5 right-1.5 flex h-6 w-6 items-center justify-center rounded-full bg-black/60 text-white hover:bg-black/80 transition-colors"
                  aria-label="Remove image"
                >
                  <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
                    <path d="M18 6L6 18M6 6l12 12" />
                  </svg>
                </button>
              </div>
            ))}
          </div>
        )}

        {/* Toolbar — only when expanded */}
        {expanded && (
          <div className="flex items-center justify-between border-t border-[var(--app-border-inner)] px-4 py-3">
            <div className="flex items-center gap-3">
              {/* Image picker */}
              {images.length < MAX_IMAGES && (
                <button
                  type="button"
                  onClick={() => fileInputRef.current?.click()}
                  className="text-[var(--app-text-muted)] hover:text-[var(--app-accent)] transition-colors"
                  title="Attach image"
                  aria-label="Attach image"
                >
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                    <rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
                    <circle cx="8.5" cy="8.5" r="1.5" />
                    <polyline points="21 15 16 10 5 21" />
                  </svg>
                </button>
              )}
              <input
                ref={fileInputRef}
                type="file"
                accept="image/jpeg,image/png,image/gif,image/webp,image/avif"
                multiple
                className="hidden"
                onChange={handleFileChange}
                aria-hidden="true"
              />

              {/* Char counter + ring */}
              {text.length > 0 && (
                <>
                  <span className={`text-[11px] font-mono tabular-nums ${charCls}`}>
                    {remaining}
                  </span>
                  <svg width="20" height="20" viewBox="0 0 20 20" className="rotate-[-90deg]" aria-hidden="true">
                    <circle cx="10" cy="10" r="8" fill="none" stroke="var(--app-border-inner)" strokeWidth="2" />
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

              {error && <span className="text-xs text-red-500">{error}</span>}
            </div>

            {/* Actions */}
            <div className="flex items-center gap-2">
              {hasContent && (
                <button
                  type="button"
                  onClick={handleDiscard}
                  className="text-xs text-[var(--app-text-muted)] hover:text-[var(--app-text-secondary)] transition-colors px-2 py-1"
                >
                  Discard
                </button>
              )}
              <button
                type="button"
                onClick={handleSubmit}
                disabled={!canPost}
                className="px-4 py-1.5 rounded-xl bg-[var(--app-accent)] text-white text-xs font-bold tracking-wide disabled:opacity-40 disabled:cursor-not-allowed hover:opacity-90 transition-opacity"
              >
                {submitting ? "Posting…" : "Post"}
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Keyboard hint */}
      {expanded && (
        <p className="mt-1.5 px-1 text-[10px] text-[var(--app-text-dim)]">
          ⌘ Return to post · Esc to collapse
        </p>
      )}
    </div>
  );
}
