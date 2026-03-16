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
    hour: "2-digit",
    minute: "2-digit",
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
  const displayTime = prettyDate(article.publishedAt, "2026年2月21日 下午09:22");
  const authorSeed = (displayName[0] || "A").toUpperCase();

  return (
    <ForumShell>
      <Link href="/" className="mb-5 inline-flex items-center gap-2 text-base text-[#d3d8e2] hover:text-white">
        ←　返回列表
      </Link>

      <article className="rounded-2xl border border-[#333944] bg-gradient-to-b from-[#20232d] to-[#171a22] p-7">
        <div className="mb-3 flex items-start justify-between">
          <h1 className="font-serif text-4xl text-[#f1f3f8]">{article.title}</h1>
          <SignatureBadge info={article.signatureInfo} />
        </div>

        <div className="mb-6 flex flex-wrap items-center gap-4">
          <span className="flex h-10 w-10 items-center justify-center rounded-full bg-[#f09a45] font-semibold text-black">
            {authorSeed}
          </span>
          <div>
            <Link href={`/@${article.author.username}`} className="text-xl font-semibold hover:text-white">
              {displayName}
            </Link>
            <p className="text-sm text-[#b9bfcb]">@{article.author.username}</p>
          </div>
          <span className="text-[#c7ccd6]">◷ {displayTime}</span>
          <Link href={`/@${ownerUsername}`} className="text-sm text-[#c7ccd6] hover:text-white">
            來自 {article.board.name}
          </Link>
        </div>

        {html ? (
          <div
            className="markdown-body border-b border-[#2f3540] pb-6"
            dangerouslySetInnerHTML={{ __html: html }}
          />
        ) : (
          <p className="border-b border-[#2f3540] pb-6 text-base text-[#d5d9e2]">尚未提供內容。</p>
        )}
      </article>

      <section className="mt-8">
        <h2 className="mb-4 font-serif text-3xl">
          回覆 ({comments.filter((c) => !c.parentId).length})
        </h2>
        {commentsLoadError && (
          <div className="mb-3 rounded-md border border-[#3b3030] bg-[#2a1f1f] p-3 text-sm text-[#f0c0c0]">
            留言資料暫時讀取失敗，請稍後重整。
          </div>
        )}
        <CommentThread comments={comments as Comment[]} articleId={article.id} />
      </section>

      <ArticleCommentForm articleId={article.id} />
    </ForumShell>
  );
}
