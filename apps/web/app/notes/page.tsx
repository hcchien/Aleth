import Link from "next/link";
import { gql } from "@/lib/gql";
import { ForumShell } from "../components/forum-shell";

const NOTES_QUERY = `
  query Notes($limit: Int) {
    notes(limit: $limit) {
      items {
        id
        kind
        noteTitle
        noteCover
        noteSummary
        content
        createdAt
        author { id username displayName }
      }
      hasMore
    }
  }
`;

interface NoteAuthor {
  id: string;
  username: string;
  displayName: string | null;
}

interface Note {
  id: string;
  kind: string;
  noteTitle: string | null;
  noteCover: string | null;
  noteSummary: string | null;
  content: string;
  createdAt: string;
  author: NoteAuthor;
}

interface NotesResponse {
  notes: { items: Note[]; hasMore: boolean };
}

function stripHtml(html: string): string {
  return html.replace(/<[^>]+>/g, " ").replace(/\s+/g, " ").trim();
}

function readingTime(html: string): number {
  const words = stripHtml(html).split(/\s+/).filter(Boolean).length;
  return Math.max(1, Math.ceil(words / 200));
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleDateString("zh-TW", { year: "numeric", month: "numeric", day: "numeric" });
}

export default async function NotesPage() {
  let notes: Note[] = [];
  try {
    const data = await gql<NotesResponse>(NOTES_QUERY, { limit: 24 }, { revalidate: 30, tags: ["notes"] });
    notes = data.notes.items;
  } catch {
    notes = [];
  }

  return (
    <ForumShell activeTab="notes">
      <div className="mb-8 flex items-center justify-between">
        <h1
          className="font-serif text-3xl font-bold text-[var(--app-text-heading)]"
          style={{ fontFamily: "var(--font-playfair), 'Playfair Display', Georgia, serif" }}
        >
          Notes
        </h1>
        <Link href="/notes/new" className="btn-outline text-sm">
          ✎ 寫 Note
        </Link>
      </div>

      {notes.length === 0 ? (
        <div className="rounded-xl border border-[var(--app-border)] bg-[var(--app-surface)] px-6 py-12 text-center text-sm text-[var(--app-text-muted)]">
          尚無 Notes。
        </div>
      ) : (
        <div className="space-y-0">
          {notes.map((note) => {
            const title = note.noteTitle ?? "（無標題）";
            const summary = note.noteSummary ?? stripHtml(note.content).slice(0, 140);
            const authorName = note.author.displayName ?? note.author.username;
            const mins = readingTime(note.content);
            const kindLabel = note.kind === "essay" ? "ESSAY"
              : note.kind === "thread" ? "THREAD"
              : "NOTE";

            return (
              <Link
                key={note.id}
                href={`/notes/${note.id}`}
                className="article-row group"
              >
                <p className="article-row__rubric rubric">{kindLabel}</p>
                <h2 className="article-row__title">{title}</h2>
                {summary && (
                  <p className="mt-1 text-sm text-[var(--app-text-secondary)] line-clamp-2 leading-relaxed">
                    {summary}
                  </p>
                )}
                <div className="article-row__meta mt-2">
                  <span>{authorName}</span>
                  <span>·</span>
                  <span>{mins} 分鐘閱讀</span>
                  <span>·</span>
                  <span>{formatDate(note.createdAt)}</span>
                </div>
              </Link>
            );
          })}
        </div>
      )}
    </ForumShell>
  );
}
