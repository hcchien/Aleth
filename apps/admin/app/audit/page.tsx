import { adminGql } from "@/lib/gql";
import { getAdminToken } from "@/lib/auth";

const AUDIT_QUERY = `
  query AuditLog($after: String, $limit: Int) {
    auditLog(after: $after, limit: $limit) {
      hasMore nextCursor
      items {
        id adminUsername action targetType targetId note metadata createdAt
      }
    }
  }
`;

interface AuditEntry {
  id: string; adminUsername: string; action: string;
  targetType: string; targetId: string; note: string | null;
  metadata: string | null; createdAt: string;
}

interface AuditData { auditLog: { hasMore: boolean; nextCursor: string | null; items: AuditEntry[] } }

const ACTION_COLORS: Record<string, string> = {
  delete_post:     "bg-red-50 text-red-600",
  resolve_report:  "bg-green-50 text-green-700",
  dismiss_report:  "bg-zinc-100 text-zinc-500",
  set_trust_level: "bg-blue-50 text-blue-700",
  suspend_user:    "bg-orange-50 text-orange-600",
  unsuspend_user:  "bg-green-50 text-green-700",
};

export default async function AuditPage({
  searchParams,
}: {
  searchParams: Promise<{ after?: string }>;
}) {
  const { after } = await searchParams;
  const token = await getAdminToken();
  let data: AuditData | null = null;
  try {
    data = await adminGql<AuditData>(AUDIT_QUERY, { after, limit: 50 }, token ?? undefined);
  } catch { /* show empty */ }

  const entries = data?.auditLog.items ?? [];

  return (
    <div>
      <h1 className="mb-6 text-2xl font-semibold">Audit Log</h1>
      <div className="rounded-lg border border-zinc-200 bg-white overflow-hidden">
        <table className="w-full text-sm">
          <thead className="border-b border-zinc-100 bg-zinc-50 text-xs font-medium uppercase tracking-wider text-zinc-400">
            <tr>
              <th className="px-4 py-3 text-left">When</th>
              <th className="px-4 py-3 text-left">Admin</th>
              <th className="px-4 py-3 text-left">Action</th>
              <th className="px-4 py-3 text-left">Target</th>
              <th className="px-4 py-3 text-left">Note / Metadata</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-zinc-100">
            {entries.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-zinc-400">No audit entries.</td>
              </tr>
            ) : entries.map((e) => (
              <tr key={e.id} className="hover:bg-zinc-50 transition-colors">
                <td className="px-4 py-3 text-xs text-zinc-400 whitespace-nowrap">
                  {new Date(e.createdAt).toLocaleString()}
                </td>
                <td className="px-4 py-3 font-medium text-zinc-700">@{e.adminUsername}</td>
                <td className="px-4 py-3">
                  <span className={`rounded px-2 py-0.5 text-xs font-medium ${ACTION_COLORS[e.action] ?? "bg-zinc-100 text-zinc-600"}`}>
                    {e.action.replace(/_/g, " ")}
                  </span>
                </td>
                <td className="px-4 py-3 text-xs text-zinc-500">
                  <span className="rounded bg-zinc-100 px-1.5 py-0.5 mr-1">{e.targetType}</span>
                  <span className="font-mono">{e.targetId.slice(0, 8)}…</span>
                </td>
                <td className="px-4 py-3 text-xs text-zinc-400 max-w-xs">
                  {e.note && <span>{e.note}</span>}
                  {e.metadata && <span className="ml-2 font-mono text-zinc-300">{e.metadata}</span>}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {data?.auditLog.hasMore && (
        <div className="mt-4 text-center">
          <a
            href={`/audit?after=${data.auditLog.nextCursor}`}
            className="rounded-md border border-zinc-200 bg-white px-4 py-2 text-sm text-zinc-600 hover:bg-zinc-50"
          >
            Load more
          </a>
        </div>
      )}
    </div>
  );
}
