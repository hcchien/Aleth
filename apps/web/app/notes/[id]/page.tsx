import { notFound } from "next/navigation";
import Link from "next/link";
import { gql } from "@/lib/gql";
import { ForumShell } from "@/app/components/forum-shell";
import { LikeButton } from "@/app/components/like-button";
import { NoteBody } from "./note-body";

const NOTE_QUERY = `
  query Note($id: ID!) {
    post(id: $id) {
      id
      kind
      noteTitle
      noteCover
      noteSummary
      content
      likeCount
      replyCount
      viewerEmotion
      reactionCounts { emotion count }
      createdAt
      signatureInfo { isSigned isVerified explanation }
      author { id username displayName trustLevel }
    }
  }
`;

interface NoteAuthor {
  id: string;
  username: string;
  displayName: string | null;
  trustLevel: number;
}

interface Note {
  id: string;
  kind: string;
  noteTitle: string | null;
  noteCover: string | null;
  noteSummary: string | null;
  content: string;
  likeCount: number;
  replyCount: number;
  viewerEmotion: string | null;
  reactionCounts: { emotion: string; count: number }[];
  createdAt: string;
  signatureInfo: { isSigned: boolean; isVerified: boolean; explanation: string };
  author: NoteAuthor;
}

interface NoteResponse {
  post: Note | null;
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
  return d.toLocaleDateString("zh-TW", { year: "numeric", month: "long", day: "numeric" });
}

export default async function NoteDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;

  let note: Note | null = null;
  try {
    const data = await gql<NoteResponse>(NOTE_QUERY, { id }, { revalidate: 30 });
    note = data.post;
  } catch {
    note = null;
  }

  if (!note || note.kind !== "note") notFound();

  const authorName = note.author.displayName ?? note.author.username;
  const mins = readingTime(note.content);

  return (
    <ForumShell activeTab="notes">
      <article className="mx-auto max-w-2xl">
        {/* Cover */}
        {note.noteCover && (
          <div className="mb-8 overflow-hidden rounded-xl">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={note.noteCover}
              alt={note.noteTitle ?? "封面"}
              className="h-64 w-full object-cover"
            />
          </div>
        )}

        {/* Title */}
        <h1 className="mb-4 font-serif text-3xl leading-snug text-[#f3f5f9] md:text-4xl">
          {note.noteTitle ?? "（無標題）"}
        </h1>

        {/* Meta */}
        <div className="mb-8 flex flex-wrap items-center gap-3 text-sm text-[#9ea4b0]">
          <Link href={`/@${note.author.username}`} className="font-medium text-[#e6e7ea] hover:text-white">
            {authorName}
          </Link>
          <span>·</span>
          <span>{mins} 分鐘閱讀</span>
          <span>·</span>
          <span>{formatDate(note.createdAt)}</span>
        </div>

        {/* Summary callout */}
        {note.noteSummary && (
          <blockquote className="mb-8 border-l-4 border-[#e89246]/50 pl-4 italic text-[#b0b8c8]">
            {note.noteSummary}
          </blockquote>
        )}

        {/* Body */}
        <NoteBody html={note.content} />

        {/* Reactions */}
        <div className="mt-10 border-t border-[#2b2f37] pt-6 flex items-center gap-4">
          <LikeButton
            postId={note.id}
            initialViewerEmotion={note.viewerEmotion}
            initialReactionCounts={note.reactionCounts}
          />
          <span className="text-sm text-[#9ea4b0]">◔ {note.replyCount}</span>
        </div>
      </article>
    </ForumShell>
  );
}
