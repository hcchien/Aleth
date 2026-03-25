import { adminGql } from "@/lib/gql";
import { getAdminToken } from "@/lib/auth";
import { ReportActions } from "./report-actions";

const QUEUE_QUERY = `
  query ReportQueue($after: String, $limit: Int) {
    reportQueue(after: $after, limit: $limit) {
      hasMore nextCursor
      items {
        postId postContent postDeleted authorId authorUsername
        reportCount oldestReport
        reports { id reason note reporterUsername createdAt }
      }
    }
  }
`;

interface ReportGroup {
  postId: string; postContent: string; postDeleted: boolean;
  authorId: string; authorUsername: string;
  reportCount: number; oldestReport: string;
  reports: { id: string; reason: string; note: string | null; reporterUsername: string; createdAt: string }[];
}

interface QueueData {
  reportQueue: { hasMore: boolean; nextCursor: string | null; items: ReportGroup[] };
}

const REASON_LABELS: Record<string, string> = {
  spam: "Spam", harassment: "Harassment", hate_speech: "Hate speech",
  misinformation: "Misinformation", illegal_content: "Illegal content", other: "Other",
};

export default async function ReportsPage({
  searchParams,
}: {
  searchParams: Promise<{ after?: string }>;
}) {
  const { after } = await searchParams;
  const token = await getAdminToken();
  let data: QueueData | null = null;
  try {
    data = await adminGql<QueueData>(QUEUE_QUERY, { after, limit: 20 }, token ?? undefined);
  } catch { /* show empty state */ }

  const groups = data?.reportQueue.items ?? [];

  return (
    <div>
      <h1 className="mb-6 text-2xl font-semibold">Reports Queue</h1>
      {groups.length === 0 ? (
        <div className="rounded-lg border border-zinc-200 bg-white p-10 text-center text-sm text-zinc-400">
          No open reports. ✓
        </div>
      ) : (
        <div className="space-y-4">
          {groups.map((g) => (
            <div key={g.postId} className="rounded-lg border border-zinc-200 bg-white overflow-hidden">
              {/* Post header */}
              <div className="flex items-center justify-between border-b border-zinc-100 px-5 py-3">
                <div className="flex items-center gap-2 text-sm">
                  <span className="font-medium text-zinc-700">@{g.authorUsername}</span>
                  {g.postDeleted && (
                    <span className="rounded bg-zinc-100 px-1.5 py-0.5 text-xs text-zinc-400">[deleted]</span>
                  )}
                  <span className="text-zinc-400">·</span>
                  <span className="text-zinc-400">{g.reportCount} report{g.reportCount !== 1 ? "s" : ""}</span>
                </div>
                <ReportActions postId={g.postId} />
              </div>

              {/* Post content */}
              <div className="px-5 py-4">
                <p className="text-sm leading-relaxed text-zinc-700 line-clamp-4 whitespace-pre-wrap">
                  {g.postContent}
                </p>
              </div>

              {/* Individual reports */}
              <div className="border-t border-zinc-100 divide-y divide-zinc-100">
                {g.reports.map((r) => (
                  <div key={r.id} className="flex items-start gap-3 px-5 py-3">
                    <span className="inline-block rounded bg-red-50 px-2 py-0.5 text-xs font-medium text-red-600">
                      {REASON_LABELS[r.reason] ?? r.reason}
                    </span>
                    {r.note && <span className="flex-1 text-xs text-zinc-500">{r.note}</span>}
                    <span className="ml-auto text-xs text-zinc-400">@{r.reporterUsername}</span>
                    <span className="text-xs text-zinc-300">
                      {new Date(r.createdAt).toLocaleDateString()}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          ))}

          {data?.reportQueue.hasMore && (
            <div className="text-center">
              <a
                href={`/reports?after=${data.reportQueue.nextCursor}`}
                className="rounded-md border border-zinc-200 bg-white px-4 py-2 text-sm text-zinc-600 hover:bg-zinc-50"
              >
                Load more
              </a>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
