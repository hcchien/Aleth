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
      author { username displayName }
      board { name }
    }
  }
`;

const ARTICLE_COMMENTS_QUERY = `
  query ArticleComments($id: ID!) {
    articleComments(articleId: $id, limit: 200) {
      id
      content
      createdAt
      parentId
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
  author: { username: string; displayName: string | null };
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
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(
    value
  );
}

function prettyDate(dateInput: string | null, fallback: string) {
  if (!dateInput) return fallback;
  const d = new Date(dateInput);
  if (Number.isNaN(d.getTime())) return fallback;
  return d.toLocaleString("zh-TW", {
    year: "numeric",
    month: "long",
    day: "numeric",
  });
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
      if (matched) {
        resolvedArticleID = matched.id;
      }
    } catch {
      // fallback to original articleId
    }
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
  } catch {
    // gateway not available
  }

  if (!article) {
    notFound();
  }

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
  const displayTime = prettyDate(article.publishedAt, "");
  const authorSeed = (displayName[0] || "A").toUpperCase();

  return (
    <ForumShell>
      {/* Back link */}
      <Link
        href="/"
        className="mb-6 inline-flex items-center gap-1.5 text-sm text-[var(--app-text-muted)] hover:text-[var(--app-accent)] transition-colors"
      >
        ← 返回列表
      </Link>

      <article className="mx-auto max-w-2xl">
        {/* Board rubric */}
        <p className="rubric mb-3">
          <Link href={`/@${ownerUsername}`} className="hover:opacity-80 transition-opacity">
            {article.board.name}
          </Link>
        </p>

        {/* Title */}
        <h1
          className="mb-6 font-serif text-4xl font-bold leading-tight text-[var(--app-text-heading)]"
          style={{ fontFamily: "var(--font-playfair), 'Playfair Display', Georgia, serif" }}
        >
          {article.title}
        </h1>

        {/* Byline */}
        <div className="mb-8 flex flex-wrap items-center gap-4 border-b border-[var(--app-border)] pb-6">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-[var(--app-accent-bg)] text-base font-semibold text-[var(--app-accent)]">
            {authorSeed}
          </div>
          <div>
            <Link
              href={`/@${article.author.username}`}
              className="text-sm font-semibold text-[var(--app-text-heading)] hover:text-[var(--app-accent)] transition-colors"
            >
              {displayName}
            </Link>
            <p className="text-xs text-[var(--app-text-muted)]">@{article.author.username}</p>
          </div>
          {displayTime && (
            <span className="text-sm text-[var(--app-text-muted)]">{displayTime}</span>
          )}
          <div className="ml-auto">
            <SignatureBadge info={article.signatureInfo} />
          </div>
        </div>

        {/* Body */}
        {html ? (
          <div
            className="markdown-body drop-cap"
            dangerouslySetInnerHTML={{ __html: html }}
          />
        ) : (
          <p className="text-base text-[var(--app-text-muted)]">尚未提供內容。</p>
        )}
      </article>

      {/* Comments */}
      <section className="mx-auto mt-12 max-w-2xl border-t border-[var(--app-border)] pt-8">
        <h2
          className="mb-5 font-serif text-2xl font-bold text-[var(--app-text-heading)]"
          style={{ fontFamily: "var(--font-playfair), 'Playfair Display', Georgia, serif" }}
        >
          回覆 ({comments.filter((c) => !c.parentId).length})
        </h2>
        {commentsLoadError && (
          <div className="mb-4 rounded-md border border-[var(--app-accent-border)] bg-[var(--app-accent-bg)] p-3 text-sm text-[var(--app-accent)]">
            留言資料暫時讀取失敗，請稍後重整。
          </div>
        )}
        <CommentThread comments={comments as Comment[]} articleId={article.id} />
        <ArticleCommentForm articleId={article.id} />
      </section>
    </ForumShell>
  );
}
