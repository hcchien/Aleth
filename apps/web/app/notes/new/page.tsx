"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { gqlClient } from "@/lib/gql-client";
import { TiptapEditor } from "@/app/components/tiptap-editor";

const CREATE_NOTE = `
  mutation CreateNote($input: CreateNoteInput!) {
    createNote(input: $input) {
      id
      kind
      noteTitle
    }
  }
`;

export default function NewNotePage() {
  const { user, loading } = useAuth();
  const router = useRouter();

  const [title, setTitle] = useState("");
  const [cover, setCover] = useState("");
  const [summary, setSummary] = useState("");
  const [content, setContent] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!loading && !user) {
      router.replace("/login");
    }
  }, [loading, user, router]);

  if (loading || !user) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-[#0b0d12] text-[#9ea4b0]">
        載入中…
      </div>
    );
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!title.trim()) { setError("請輸入標題"); return; }
    if (!content || content === "<p></p>") { setError("請輸入內容"); return; }

    setSubmitting(true);
    setError("");
    try {
      const input: Record<string, string> = {
        content,
        noteTitle: title.trim(),
      };
      if (cover.trim()) input.noteCover = cover.trim();
      if (summary.trim()) input.noteSummary = summary.trim();

      const data = await gqlClient<{ createNote: { id: string } }>(CREATE_NOTE, { input });
      router.push(`/notes/${data.createNote.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "發布失敗，請稍後再試");
      setSubmitting(false);
    }
  }

  return (
    <div className="min-h-screen bg-[#0b0d12] text-[#e6e7ea]">
      <header className="sticky top-0 z-20 border-b border-[#2b2f37] bg-[#0f1118] px-4 py-4">
        <div className="mx-auto flex max-w-3xl items-center justify-between">
          <h1 className="font-serif text-xl text-[#f09a45]">新增 Note</h1>
          <div className="flex gap-3">
            <button
              type="button"
              onClick={() => router.back()}
              className="rounded-full border border-[#3a3f48] px-4 py-1.5 text-sm text-[#9ea4b0] hover:text-white transition-colors"
            >
              取消
            </button>
            <button
              type="submit"
              form="note-form"
              disabled={submitting}
              className="rounded-full bg-[#e89246] px-4 py-1.5 text-sm font-medium text-[#0e1117] hover:bg-[#d4843e] disabled:opacity-50 transition-colors"
            >
              {submitting ? "發布中…" : "發布"}
            </button>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-3xl px-4 py-8">
        <form id="note-form" onSubmit={handleSubmit} className="space-y-5">
          {error && (
            <div className="rounded-lg border border-red-800/50 bg-red-950/30 px-4 py-2 text-sm text-red-400">
              {error}
            </div>
          )}

          <div>
            <label className="mb-1.5 block text-sm text-[#9ea4b0]">
              標題 <span className="text-red-400">*</span>
            </label>
            <input
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Note 標題"
              required
              className="w-full rounded-lg border border-[#3a3f48] bg-[#161b22] px-4 py-2.5 text-base text-white placeholder-[#5a6070] focus:border-[#e89246]/50 focus:outline-none"
            />
          </div>

          <div>
            <label className="mb-1.5 block text-sm text-[#9ea4b0]">封面圖片 URL（選填）</label>
            <input
              type="url"
              value={cover}
              onChange={(e) => setCover(e.target.value)}
              placeholder="https://example.com/cover.jpg"
              className="w-full rounded-lg border border-[#3a3f48] bg-[#161b22] px-4 py-2.5 text-sm text-white placeholder-[#5a6070] focus:border-[#e89246]/50 focus:outline-none"
            />
          </div>

          <div>
            <label className="mb-1.5 block text-sm text-[#9ea4b0]">
              摘要（選填，最多 300 字）
            </label>
            <textarea
              value={summary}
              onChange={(e) => setSummary(e.target.value)}
              maxLength={300}
              rows={2}
              placeholder="一句話介紹這篇 Note"
              className="w-full resize-none rounded-lg border border-[#3a3f48] bg-[#161b22] px-4 py-2.5 text-sm text-white placeholder-[#5a6070] focus:border-[#e89246]/50 focus:outline-none"
            />
          </div>

          <div>
            <label className="mb-1.5 block text-sm text-[#9ea4b0]">
              內容 <span className="text-red-400">*</span>
            </label>
            <TiptapEditor value={content} onChange={setContent} />
          </div>
        </form>
      </main>
    </div>
  );
}
