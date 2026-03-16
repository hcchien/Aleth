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
        <h1 className="font-serif text-4xl">Notes</h1>
        <Link
          href="/notes/new"
          className="rounded-full border border-[#e89246]/50 bg-[#2a1f18] px-4 py-1.5 text-sm text-[#e89246] hover:bg-[#3a2a18] transition-colors"
        >
          ✎ 寫 Note
        </Link>
      </div>

      {notes.length === 0 ? (
        <div className="rounded-xl border border-[#333944] bg-[#0f1117] px-6 py-10 text-center text-sm text-[#aeb4bf]">
          尚無 Notes。
        </div>
      ) : (
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {notes.map((note) => {
            const title = note.noteTitle ?? "（無標題）";
            const summary = note.noteSummary ?? stripHtml(note.content).slice(0, 140);
            const authorName = note.author.displayName ?? note.author.username;
            const mins = readingTime(note.content);

            return (
              <Link
                key={note.id}
                href={`/notes/${note.id}`}
                className="group flex flex-col rounded-2xl border border-[#333944] bg-[#0f1117] overflow-hidden hover:border-[#4a5060] transition-colors"
              >
                {note.noteCover && (
                  <div className="aspect-video overflow-hidden">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img
                      src={note.noteCover}
                      alt={title}
                      className="h-full w-full object-cover group-hover:scale-105 transition-transform duration-300"
                    />
                  </div>
                )}
                <div className="flex flex-col gap-2 p-5 flex-1">
                  <h2 className="font-serif text-lg leading-snug text-[#f3f5f9] group-hover:text-white line-clamp-2">
                    {title}
                  </h2>
                  {summary && (
                    <p className="text-sm text-[#9ea4b0] line-clamp-3 flex-1">{summary}</p>
                  )}
                  <div className="mt-2 flex items-center gap-2 text-xs text-[#6b7280]">
                    <span>{authorName}</span>
                    <span>·</span>
                    <span>{mins} 分鐘</span>
                    <span>·</span>
                    <span>{formatDate(note.createdAt)}</span>
                  </div>
                </div>
              </Link>
            );
          })}
        </div>
      )}
    </ForumShell>
  );
}
