import Link from "next/link";

function formatDate(iso: string | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleDateString("zh-TW", { month: "numeric", day: "numeric" });
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
}

export function ArticleCard({ article }: ArticleCardProps) {
  const ownerUsername = article.board.owner.username.replace(/^@+/, "");
  const articleRef = article.slug || article.id;
  const href = `/@${ownerUsername}/${articleRef}`;
  const authorName = article.author.displayName ?? article.author.username;
  const date = formatDate(article.publishedAt);

  return (
    <article className="rounded-xl border border-[#2a2e38] bg-[#0f1117] p-5 transition-colors hover:border-[#404654]">
      <Link href={href} className="group">
        <h2 className="mb-2 font-serif text-lg text-[#f3c06a] group-hover:text-amber-300">
          {article.title}
        </h2>
      </Link>
      <div className="flex items-center gap-2 text-xs text-[#7a8090]">
        <Link
          href={`/@${ownerUsername}`}
          className="font-medium text-[#aeb4bf] hover:text-white"
        >
          {article.board.name}
        </Link>
        <span>·</span>
        <span>{authorName}</span>
        {date && (
          <>
            <span>·</span>
            <span>{date}</span>
          </>
        )}
      </div>
    </article>
  );
}
