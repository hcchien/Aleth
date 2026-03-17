import { notFound } from "next/navigation";
import { getTranslations } from "next-intl/server";
import Link from "next/link";
import { gql } from "@/lib/gql";
import { ForumShell } from "../../../components/forum-shell";
import { ArticleCard } from "../../../components/article-card";

const SERIES_QUERY = `
  query Series($id: ID!) {
    series(id: $id) {
      id
      boardId
      title
      description
      articleCount
      articles {
        id title slug publishedAt createdAt status accessPolicy
        author { id username displayName }
        board { id name owner { username displayName } }
        signatureInfo { isSigned isVerified explanation }
      }
    }
  }
`;

interface SeriesData {
  id: string;
  boardId: string;
  title: string;
  description: string | null;
  articleCount: number;
  articles: ArticleItem[];
}

interface ArticleItem {
  id: string;
  title: string;
  slug: string;
  publishedAt: string | null;
  createdAt: string;
  status: string;
  accessPolicy: string;
  author: { id: string; username: string; displayName: string | null };
  board: { id: string; name: string; owner: { username: string; displayName: string | null } };
  signatureInfo: { isSigned: boolean; isVerified: boolean; explanation: string };
}

interface PageProps {
  params: Promise<{ username: string; seriesId: string }>;
}

export default async function SeriesPage({ params }: PageProps) {
  const { username, seriesId } = await params;
  const t = await getTranslations("series");

  let series: SeriesData | null = null;
  try {
    const data = await gql<{ series: SeriesData | null }>(
      SERIES_QUERY,
      { id: seriesId },
      { revalidate: 60 }
    );
    series = data.series;
  } catch {
    // gateway unavailable
  }

  if (!series) notFound();

  const publishedArticles = series.articles.filter((a) => a.status === "published");

  return (
    <ForumShell>
      {/* Header */}
      <div className="mb-8">
        <p className="mb-1 text-sm text-[var(--app-text-muted)]">
          <Link href={`/@${username}`} className="hover:text-[var(--app-text)] transition-colors">
            @{username}
          </Link>
          {" › "}
          <span>{t("series")}</span>
        </p>
        <h1 className="font-serif text-3xl text-[var(--app-text-heading)]">{series.title}</h1>
        {series.description && (
          <p className="mt-2 max-w-2xl text-sm leading-relaxed text-[var(--app-text-muted)]">
            {series.description}
          </p>
        )}
        <p className="mt-2 text-xs text-[var(--app-text-secondary)]">
          {t("articleCount", { count: series.articleCount })}
        </p>
      </div>

      {/* Article list */}
      {publishedArticles.length === 0 ? (
        <div className="rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)] px-6 py-10 text-center text-sm text-[var(--app-text-muted)]">
          {t("seriesEmpty")}
        </div>
      ) : (
        <div className="space-y-4">
          {publishedArticles.map((article, idx) => (
            <div key={article.id} className="flex gap-4">
              <div className="flex w-8 shrink-0 flex-col items-center pt-5">
                <span className="font-mono text-sm font-medium text-[var(--app-text-muted)]">
                  {idx + 1}
                </span>
                {idx < publishedArticles.length - 1 && (
                  <div className="mt-2 flex-1 border-l border-dashed border-[var(--app-border)]" />
                )}
              </div>
              <div className="flex-1 min-w-0">
                <ArticleCard article={article} />
              </div>
            </div>
          ))}
        </div>
      )}
    </ForumShell>
  );
}
