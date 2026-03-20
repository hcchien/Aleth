import Link from "next/link";

function formatDate(iso: string | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleDateString("zh-TW", { year: "numeric", month: "numeric", day: "numeric" });
}

interface ArticleCardProps {
  article: {
    id: string;
    title: string;
    slug: string;
    publishedAt: string | null;
    author: {
      username: string;
      displayName: string | null;
    };
    board: {
      name: string;
      owner: { username: string };
    };
  };
  /** Optional rubric label shown above the title */
  rubric?: string;
}

export function ArticleCard({ article, rubric }: ArticleCardProps) {
  const ownerUsername = article.board.owner.username.replace(/^@+/, "");
  const articleRef = article.slug || article.id;
  const href = `/@${ownerUsername}/${articleRef}`;
  const authorName = article.author.displayName ?? article.author.username;
  const date = formatDate(article.publishedAt);

  return (
    <Link href={href} className="article-row group block">
      {rubric && (
        <p className="article-row__rubric rubric">{rubric}</p>
      )}
      <h2 className="article-row__title">{article.title}</h2>
      <div className="article-row__meta">
        <span className="font-medium text-[var(--app-text-secondary)]">
          {article.board.name}
        </span>
        <span>·</span>
        <span>{authorName}</span>
        {date && (
          <>
            <span>·</span>
            <span>{date}</span>
          </>
        )}
      </div>
    </Link>
  );
}
