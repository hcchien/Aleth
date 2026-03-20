"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/lib/auth-context";
import { gqlClient } from "@/lib/gql-client";
import { CreateSeriesModal } from "@/app/components/create-series-modal";

const BOARD_SERIES_QUERY = `
  query MyBoardSeries($username: String!) {
    board(username: $username) {
      id
      series {
        id boardId title description articleCount createdAt
      }
    }
  }
`;

const UPDATE_SERIES = `
  mutation UpdateSeries($id: ID!, $title: String!, $description: String) {
    updateSeries(id: $id, input: { title: $title, description: $description }) {
      id title description
    }
  }
`;

const DELETE_SERIES = `
  mutation DeleteSeries($id: ID!) {
    deleteSeries(id: $id)
  }
`;

interface Series {
  id: string;
  boardId: string;
  title: string;
  description: string | null;
  articleCount: number;
  createdAt: string;
}

interface EditState {
  id: string;
  title: string;
  description: string;
}

export default function SeriesSettingsPage() {
  const t = useTranslations("series");
  const { user, loading: authLoading } = useAuth();
  const [boardId, setBoardId] = useState<string | null>(null);
  const [seriesList, setSeriesList] = useState<Series[]>([]);
  const [fetching, setFetching] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [edit, setEdit] = useState<EditState | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (authLoading || !user) return;
    gqlClient<{ board: { id: string; series: Series[] } | null }>(
      BOARD_SERIES_QUERY,
      { username: user.username }
    )
      .then((data) => {
        if (data.board) {
          setBoardId(data.board.id);
          setSeriesList(data.board.series);
        }
      })
      .catch(() => {})
      .finally(() => setFetching(false));
  }, [user, authLoading]);

  async function handleSaveEdit() {
    if (!edit) return;
    setSaving(true);
    setError(null);
    try {
      await gqlClient(UPDATE_SERIES, {
        id: edit.id,
        title: edit.title,
        description: edit.description || undefined,
      });
      setSeriesList((prev) =>
        prev.map((s) =>
          s.id === edit.id
            ? { ...s, title: edit.title, description: edit.description || null }
            : s
        )
      );
      setEdit(null);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Unknown error");
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(id: string) {
    if (!confirm(t("deleteSeriesConfirm"))) return;
    try {
      await gqlClient(DELETE_SERIES, { id });
      setSeriesList((prev) => prev.filter((s) => s.id !== id));
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Unknown error");
    }
  }

  if (authLoading || fetching) {
    return (
      <div className="space-y-3">
        {[1, 2].map((i) => (
          <div key={i} className="h-16 animate-pulse rounded-xl bg-[var(--app-border)]" />
        ))}
      </div>
    );
  }

  if (!user) return null;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="font-serif text-xl text-[var(--app-text-heading)]">
          {t("manageSeries")}
        </h2>
        {boardId && (
          <button
            onClick={() => setShowCreate(true)}
            className="rounded-full bg-[var(--app-accent)] px-4 py-1.5 text-sm font-medium text-white hover:opacity-90 transition-opacity"
          >
            + {t("createSeries")}
          </button>
        )}
      </div>

      {error && <p className="text-xs text-red-400">{error}</p>}

      {seriesList.length === 0 ? (
        <div className="rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)] px-6 py-10 text-center text-sm text-[var(--app-text-muted)]">
          {t("noSeries")}
        </div>
      ) : (
        <div className="space-y-3">
          {seriesList.map((s) => (
            <div
              key={s.id}
              className="rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)] p-4"
            >
              {edit?.id === s.id ? (
                <div className="space-y-3">
                  <input
                    value={edit.title}
                    onChange={(e) => setEdit({ ...edit, title: e.target.value })}
                    className="w-full rounded-lg border border-[var(--app-border)] bg-[var(--app-bg)] px-3 py-2 text-sm text-[var(--app-text-bright)] focus:border-[var(--app-accent)] focus:outline-none"
                  />
                  <textarea
                    value={edit.description}
                    onChange={(e) => setEdit({ ...edit, description: e.target.value })}
                    rows={2}
                    placeholder={t("seriesDescription")}
                    className="w-full rounded-lg border border-[var(--app-border)] bg-[var(--app-bg)] px-3 py-2 text-sm text-[var(--app-text-bright)] focus:border-[var(--app-accent)] focus:outline-none resize-none"
                  />
                  <div className="flex gap-2">
                    <button
                      onClick={handleSaveEdit}
                      disabled={saving || !edit.title.trim()}
                      className="rounded-full bg-[var(--app-accent)] px-4 py-1 text-xs font-medium text-white hover:opacity-90 disabled:opacity-50"
                    >
                      {t("saveSeries")}
                    </button>
                    <button
                      onClick={() => setEdit(null)}
                      className="rounded-full px-4 py-1 text-xs text-[var(--app-text-secondary)] hover:text-[var(--app-text)]"
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              ) : (
                <div className="flex items-start justify-between gap-4">
                  <div className="min-w-0">
                    <p className="font-medium text-[var(--app-text-bright)] truncate">{s.title}</p>
                    {s.description && (
                      <p className="mt-0.5 text-xs text-[var(--app-text-muted)] line-clamp-2">
                        {s.description}
                      </p>
                    )}
                    <p className="mt-1 text-xs text-[var(--app-text-secondary)]">
                      {t("articleCount", { count: s.articleCount })}
                    </p>
                  </div>
                  <div className="flex shrink-0 gap-2">
                    <button
                      onClick={() =>
                        setEdit({ id: s.id, title: s.title, description: s.description ?? "" })
                      }
                      className="text-xs text-[var(--app-text-secondary)] hover:text-[var(--app-text)] transition-colors"
                    >
                      {t("editSeries")}
                    </button>
                    <button
                      onClick={() => handleDelete(s.id)}
                      className="text-xs text-red-400/70 hover:text-red-400 transition-colors"
                    >
                      {t("deleteSeries")}
                    </button>
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {showCreate && boardId && (
        <CreateSeriesModal
          boardId={boardId}
          onClose={() => setShowCreate(false)}
          onCreated={(created) =>
            setSeriesList((prev) => [
              { ...created, description: null, boardId: boardId!, articleCount: 0, createdAt: new Date().toISOString() },
              ...prev,
            ])
          }
        />
      )}
    </div>
  );
}
