import { adminGql } from "@/lib/gql";
import { getAdminToken } from "@/lib/auth";

const STATS_QUERY = `
  query {
    dashboardStats {
      openReports newUsers24h newUsers7d
      newPosts24h newPosts7d totalUsers totalPosts
    }
  }
`;

interface Stats {
  dashboardStats: {
    openReports: number; newUsers24h: number; newUsers7d: number;
    newPosts24h: number; newPosts7d: number; totalUsers: number; totalPosts: number;
  };
}

function StatCard({ label, value, sub }: { label: string; value: number; sub?: string }) {
  return (
    <div className="rounded-lg border border-zinc-200 bg-white p-5">
      <p className="text-xs font-medium uppercase tracking-wider text-zinc-400">{label}</p>
      <p className="mt-2 text-3xl font-semibold text-zinc-900">{value.toLocaleString()}</p>
      {sub && <p className="mt-1 text-xs text-zinc-400">{sub}</p>}
    </div>
  );
}

export default async function DashboardPage() {
  const token = await getAdminToken();
  let stats: Stats["dashboardStats"] | null = null;
  try {
    const data = await adminGql<Stats>(STATS_QUERY, {}, token ?? undefined);
    stats = data.dashboardStats;
  } catch {
    // Show empty state if service is unavailable
  }

  return (
    <div>
      <h1 className="mb-6 text-2xl font-semibold text-zinc-900">Dashboard</h1>
      {stats ? (
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <StatCard label="Open Reports" value={stats.openReports} />
          <StatCard label="Total Users" value={stats.totalUsers} />
          <StatCard label="Total Posts" value={stats.totalPosts} />
          <StatCard label="New Users (24h)" value={stats.newUsers24h} sub={`${stats.newUsers7d} this week`} />
          <StatCard label="New Posts (24h)" value={stats.newPosts24h} sub={`${stats.newPosts7d} this week`} />
        </div>
      ) : (
        <div className="rounded-lg border border-zinc-200 bg-white p-8 text-center text-sm text-zinc-400">
          Could not load stats — admin service may be unavailable.
        </div>
      )}
    </div>
  );
}
