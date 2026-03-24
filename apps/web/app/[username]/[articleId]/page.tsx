import { notFound } from "next/navigation";
import Link from "next/link";
import { gql } from "@/lib/gql";
import { renderMarkdown } from "@/lib/markdown";
import { ForumShell } from "@/app/components/forum-shell";
import { SignatureBadge, type SignatureInfo } from "@/app/components/signature-badge";
import { CommentThread, type Comment } from "@/app/components/comment-thread";
import { ArticleCommentForm } from "@/app/components/article-comment-form";

const ARTICLE_QUERY = `
  query Article($id: ID!) {
    article(id: $id) {
      id title contentMd status publishedAt updatedAt
      signatureInfo { isSigned isVerified contentHash signature algorithm explanation }
      author { username displayName trustLevel }
      board { name }
    }
  }
`;

const ARTICLE_COMMENTS_QUERY = `
  query ArticleComments($id: ID!) {
    articleComments(articleId: $id, limit: 200) {
      id content createdAt parentId
      author { username displayName }
    }
  }
`;

const BOARD_ARTICLES_LOOKUP_QUERY = `
  query BoardArticlesLookup($username: String!, $limit: Int) {
    boardArticles(username: $username, limit: $limit) {
      items { id slug }
    }
  }
`;

interface Article {
  id: string;
  title: string;
  contentMd: string | null;
  status: string;
  publishedAt: string | null;
  updatedAt: string;
  signatureInfo: SignatureInfo;
  author: { username: string; displayName: string | null; trustLevel: number };
  board: { name: string };
}

interface ArticleComment {
  id: string;
  content: string;
  createdAt: string;
  parentId: string | null;
  author: { username: string; displayName: string | null };
}

interface PageProps {
  params: Promise<{ username: string; articleId: string }>;
}

function looksLikeUUID(value: string): boolean {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value);
}

function prettyDate(dateInput: string | null) {
  if (!dateInput) return null;
  const d = new Date(dateInput);
  if (Number.isNaN(d.getTime())) return null;
  return d.toLocaleString("zh-TW", { year: "numeric", month: "long", day: "numeric" });
}

function readingTime(md: string | null): number {
  if (!md) return 1;
  const words = md.trim().split(/\s+/).length;
  return Math.max(1, Math.ceil(words / 200));
}

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

export default async function ArticlePage({ params }: PageProps) {
  const { username: rawUsername, articleId } = await params;
  const decodedUsername = decodeURIComponent(rawUsername);
  const ownerUsername = decodedUsername.startsWith("@")
    ? decodedUsername.slice(1)
    : decodedUsername.startsWith("%40")
    ? decodedUsername.slice(3)
    : decodedUsername;

  let resolvedArticleID = articleId;
  if (!looksLikeUUID(articleId)) {
    try {
      const lookup = await gql<{ boardArticles: { items: { id: string; slug: string }[] } }>(
        BOARD_ARTICLES_LOOKUP_QUERY,
        { username: ownerUsername, limit: 200 },
        { revalidate: 0 }
      );
      const matched = lookup.boardArticles.items.find(
        (a) => a.slug === articleId || a.id === articleId
      );
      if (matched) resolvedArticleID = matched.id;
    } catch { /* fallback */ }
  }

  let article: Article | null = null;
  let comments: ArticleComment[] = [];
  let commentsLoadError = false;

  try {
    const data = await gql<{ article: Article | null }>(
      ARTICLE_QUERY,
      { id: resolvedArticleID },
      { revalidate: 120 }
    );
    article = data.article;
  } catch { /* gateway not available */ }

  if (!article) notFound();

  try {
    const commentsData = await gql<{ articleComments: ArticleComment[] }>(
      ARTICLE_COMMENTS_QUERY,
      { id: resolvedArticleID },
      { revalidate: 0, tags: [`article-comments:${articleId}`] }
    );
    comments = commentsData.articleComments ?? [];
  } catch {
    comments = [];
    commentsLoadError = true;
  }

  const html = article.contentMd ? renderMarkdown(article.contentMd) : null;
  const displayName = article.author.displayName ?? article.author.username;
  const avatarChar = (displayName[0] || "A").toUpperCase();
  const avatarCls = avatarColor(article.author.username);
  const displayTime = prettyDate(article.publishedAt);
  const mins = readingTime(article.contentMd);
  const isVerified = article.author.trustLevel >= 2;

  return (
    <ForumShell>
      {/* ── Back nav ── */}
      <Link
        href={`/@${ownerUsername}`}
        className="mb-8 inline-flex items-center gap-1.5 text-[11px] font-bold uppercase tracking-widest text-[var(--app-text-muted)] hover:text-[var(--app-accent)] transition-colors"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <path d="M19 12H5M12 5l-7 7 7 7" />
        </svg>
        {article.board.name}
      </Link>

      <article className="mx-auto max-w-2xl">

        {/* ── Header ── */}
        <header className="mb-10">
          {/* Rubric */}
          <p className="mb-3 text-[11px] font-bold uppercase tracking-widest text-[var(--app-accent)]">
            <Link href={`/@${ownerUsername}`} className="hover:opacity-70 transition-opacity">
              {article.board.name}
            </Link>
          </p>

          {/* Title */}
          <h1
            className="mb-8 text-4xl font-bold leading-tight text-[var(--app-text-heading)] tracking-tight"
            style={{ fontFamily: "var(--font-newsreader), 'Newsreader', Georgia, serif", textWrap: "balance" as const }}
          >
            {article.title}
          </h1>

          {/* Byline */}
          <div className="flex items-center gap-4 border-y border-[var(--app-border-inner)] py-5">
            {/* Avatar */}
            <Link href={`/@${article.author.username}`} className="shrink-0">
              <div className={`flex h-11 w-11 items-center justify-center rounded-xl text-base font-bold ${avatarCls}`}>
                {avatarChar}
              </div>
            </Link>

            {/* Name + meta */}
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 flex-wrap">
                <Link
                  href={`/@${article.author.username}`}
                  className="text-sm font-bold text-[var(--app-text-heading)] hover:text-[var(--app-accent)] transition-colors"
                >
                  {displayName}
                </Link>
                {isVerified && (
                  <span className="inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-tighter text-[var(--app-secondary)] bg-[var(--app-secondary-bg)]">
                    <svg width="10" height="10" viewBox="0 0 8 8" fill="currentColor" aria-hidden="true">
                      <path d="M3.5 6.5L1 4l.7-.7 1.8 1.8 3.8-3.8.7.7z" />
                    </svg>
                    Verified
                  </span>
                )}
              </div>
              <div className="flex items-center gap-2 mt-0.5 text-[11px] text-[var(--app-text-muted)]">
                {displayTime && <span>{displayTime}</span>}
                {displayTime && <span className="text-[var(--app-text-dim)]">·</span>}
                <span>{mins} min read</span>
              </div>
            </div>

            {/* Signature badge */}
            <div className="shrink-0">
              <SignatureBadge info={article.signatureInfo} />
            </div>
          </div>
        </header>

        {/* ── Body ── */}
        {html ? (
          <div className="markdown-body drop-cap" dangerouslySetInnerHTML={{ __html: html }} />
        ) : (
          <p className="text-base text-[var(--app-text-muted)] italic">尚未提供內容。</p>
        )}

        {/* ── Author card (bottom) ── */}
        <div className="mt-16 rounded-2xl border border-[var(--app-border-inner)] bg-[var(--app-surface-3)] p-6 flex items-center gap-4">
          <Link href={`/@${article.author.username}`} className="shrink-0">
            <div className={`flex h-14 w-14 items-center justify-center rounded-2xl text-xl font-bold ${avatarCls}`}>
              {avatarChar}
            </div>
          </Link>
          <div className="flex-1 min-w-0">
            <p className="text-[10px] font-bold uppercase tracking-widest text-[var(--app-text-muted)] mb-1">Written by</p>
            <Link href={`/@${article.author.username}`} className="font-bold text-[var(--app-text-heading)] hover:text-[var(--app-accent)] transition-colors">
              {displayName}
            </Link>
            <p className="text-xs text-[var(--app-text-muted)] mt-0.5">@{article.author.username}</p>
          </div>
          <Link
            href={`/@${article.author.username}`}
            className="shrink-0 px-4 py-2 rounded-xl border border-[var(--app-border-inner)] text-[11px] font-bold uppercase tracking-widest text-[var(--app-text-muted)] hover:border-[var(--app-accent)] hover:text-[var(--app-accent)] transition-all"
          >
            View Profile
          </Link>
        </div>
      </article>

      {/* ── Comments ── */}
      <section className="mx-auto mt-12 max-w-2xl border-t border-[var(--app-border-inner)] pt-10">
        <h2 className="mb-6 text-[10px] font-bold uppercase tracking-widest text-[var(--app-text-muted)]">
          Replies
          <span className="ml-2 text-[var(--app-text-dim)]">({comments.filter((c) => !c.parentId).length})</span>
        </h2>

        {commentsLoadError && (
          <div className="mb-4 rounded-xl border border-[var(--app-accent-border)] bg-[var(--app-accent-bg)] p-4 text-sm text-[var(--app-accent)]">
            留言資料暫時讀取失敗，請稍後重整。
          </div>
        )}

        <CommentThread comments={comments as Comment[]} articleId={article.id} />
        <ArticleCommentForm articleId={article.id} />
      </section>
    </ForumShell>
  );
}
