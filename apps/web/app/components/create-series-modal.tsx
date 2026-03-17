"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { gqlClient } from "@/lib/gql-client";
import { useRouter } from "next/navigation";

const CREATE_SERIES = `
  mutation CreateSeries($boardId: ID!, $title: String!, $description: String) {
    createSeries(input: { boardId: $boardId, title: $title, description: $description }) {
      id title description articleCount
    }
  }
`;

interface Props {
  boardId: string;
  onClose: () => void;
  onCreated?: (series: { id: string; title: string }) => void;
}

export function CreateSeriesModal({ boardId, onClose, onCreated }: Props) {
  const t = useTranslations("series");
  const router = useRouter();
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!title.trim()) return;
    setLoading(true);
    setError(null);
    try {
      const data = await gqlClient<{
        createSeries: { id: string; title: string };
      }>(CREATE_SERIES, {
        boardId,
        title: title.trim(),
        description: description.trim() || undefined,
      });
      onCreated?.(data.createSeries);
      router.refresh();
      onClose();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
      <div className="w-full max-w-md rounded-2xl border border-[var(--app-border-2)] bg-[var(--app-surface)] p-6 shadow-xl">
        <h2 className="mb-4 font-serif text-xl text-[var(--app-text-heading)]">
          {t("createSeries")}
        </h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="mb-1 block text-sm text-[var(--app-text-secondary)]">
              {t("seriesTitle")}
            </label>
            <input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              required
              className="w-full rounded-lg border border-[var(--app-border)] bg-[var(--app-bg)] px-3 py-2 text-sm text-[var(--app-text-bright)] focus:border-[var(--app-accent)] focus:outline-none"
            />
          </div>
          <div>
            <label className="mb-1 block text-sm text-[var(--app-text-secondary)]">
              {t("seriesDescription")}
            </label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={3}
              className="w-full rounded-lg border border-[var(--app-border)] bg-[var(--app-bg)] px-3 py-2 text-sm text-[var(--app-text-bright)] focus:border-[var(--app-accent)] focus:outline-none resize-none"
            />
          </div>
          {error && (
            <p className="text-xs text-red-400">{error}</p>
          )}
          <div className="flex justify-end gap-2 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="rounded-full px-4 py-1.5 text-sm text-[var(--app-text-secondary)] hover:text-[var(--app-text)] transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading || !title.trim()}
              className="rounded-full bg-[var(--app-accent)] px-5 py-1.5 text-sm font-medium text-white hover:opacity-90 transition-opacity disabled:opacity-50"
            >
              {loading ? "…" : t("saveSeries")}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
