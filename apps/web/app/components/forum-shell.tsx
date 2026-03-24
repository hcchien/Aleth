import Link from "next/link";
import { getTranslations } from "next-intl/server";
import { ShellAuthControls } from "./shell-auth-controls";

export type FollowedUser = {
  id: string;
  initial: string;
  name: string;
  username: string;
  tier: string;
  tierClass: string;
  trustLevel?: number;
};

export type FanPage = {
  id: string;
  icon: string;
  /** For fan pages: use slug (links to /p/{slug}). For legacy boards: use ownerUsername (links to /@{username}). */
  slug?: string;
  ownerUsername: string;
  name: string;
  count: number;
};

// ─── Shell ────────────────────────────────────────────────────────────────────

export async function ForumShell({
  children,
  followedUsers = [],
  fanPages = [],
  activeTab,
}: {
  children: React.ReactNode;
  followedUsers?: FollowedUser[];
  fanPages?: FanPage[]
  activeTab?: "feed" | "notes";
}) {
  const t = await getTranslations("sidebar");
  const tn = await getTranslations("nav");

  const trendingTags = [
    { tag: "#DecentralizedEnergy", vouches: "2.4k" },
    { tag: "#LedgerSafety", vouches: "1.8k" },
    { tag: "#GlobalLiquidity", vouches: "940" },
    { tag: "#VerifiedSources", vouches: "712" },
    { tag: "#ChainGovernance", vouches: "538" },
  ];

  return (
    <div className="bg-[var(--app-bg)] text-[var(--app-text)]">
      {/* ── Fixed Header ── */}
      <header className="app-header fixed top-0 left-0 w-full z-50 flex items-center justify-between px-4 md:px-8 h-16 border-b border-[var(--app-border-inner)]">
        <div className="flex items-center gap-8">
          {/* Wordmark */}
          <Link
            href="/"
            className="text-2xl font-black text-[var(--app-text-heading)] tracking-tighter hover:opacity-80 transition-opacity"
            style={{ fontFamily: "var(--font-manrope), 'Manrope', sans-serif" }}
          >
            Aleth
          </Link>

          {/* Top nav tabs */}
          <nav className="hidden md:flex items-center gap-6">
            <Link
              href="/"
              className={`pb-1 text-sm font-bold tracking-tight transition-colors ${
                !activeTab || activeTab === "feed"
                  ? "text-[var(--app-text-heading)] border-b-2 border-[var(--app-accent)]"
                  : "text-[var(--app-text-nav)] hover:text-[var(--app-text-heading)]"
              }`}
            >
              {tn("feed")}
            </Link>
            <Link
              href="/notes"
              className={`pb-1 text-sm font-bold tracking-tight transition-colors ${
                activeTab === "notes"
                  ? "text-[var(--app-text-heading)] border-b-2 border-[var(--app-accent)]"
                  : "text-[var(--app-text-nav)] hover:text-[var(--app-text-heading)]"
              }`}
            >
              {tn("notes")}
            </Link>
            <Link
              href="/identity"
              className="pb-1 text-sm font-bold tracking-tight text-[var(--app-text-nav)] hover:text-[var(--app-text-heading)] transition-colors"
            >
              Identity
            </Link>
          </nav>
        </div>

        {/* Right: auth controls */}
        <ShellAuthControls />
      </header>

      {/* ── Fixed Left Sidebar ── */}
      <aside className="fixed left-0 top-0 pt-16 h-screen w-64 bg-[var(--app-sidebar)] flex flex-col border-r border-[var(--app-border-inner)] overflow-y-auto hidden md:flex">
        {/* Brand node identity */}
        <div className="px-6 pt-6 pb-5">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-lg bg-[var(--app-surface-2)] flex items-center justify-center text-[var(--app-accent)] flex-shrink-0">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <path d="M12 2 L21 5.5 V11.5 C21 16.5 17 20.5 12 22 C7 20.5 3 16.5 3 11.5 V5.5 Z" />
                <path d="M8.5 12l2.5 2.5 4.5-5" />
              </svg>
            </div>
            <div>
              <p className="text-xs font-semibold text-[var(--app-text-heading)] tracking-tight">Aleth</p>
              <p className="text-[10px] text-[var(--app-text-muted)] uppercase tracking-widest">Verified Node</p>
            </div>
          </div>
        </div>

        {/* Nav */}
        <nav className="flex-1 px-4 space-y-1">
          <Link
            href="/"
            className={`flex items-center gap-3 px-4 py-3 rounded-lg text-sm font-medium transition-all duration-200 ${
              !activeTab || activeTab === "feed"
                ? "bg-[var(--app-surface-2)] text-[var(--app-secondary)]"
                : "text-[var(--app-text-nav)] hover:bg-[var(--app-surface-2)]/50 hover:text-[var(--app-text-heading)]"
            }`}
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <path d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            {tn("feed")}
          </Link>
          <Link
            href="/notes"
            className={`flex items-center gap-3 px-4 py-3 rounded-lg text-sm font-medium transition-all duration-200 ${
              activeTab === "notes"
                ? "bg-[var(--app-surface-2)] text-[var(--app-secondary)]"
                : "text-[var(--app-text-nav)] hover:bg-[var(--app-surface-2)]/50 hover:text-[var(--app-text-heading)]"
            }`}
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2" /><circle cx="9" cy="7" r="4" /><path d="M23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75" />
            </svg>
            {tn("notes")}
          </Link>
          <Link
            href="/settings"
            className="flex items-center gap-3 px-4 py-3 rounded-lg text-sm font-medium text-[var(--app-text-nav)] hover:bg-[var(--app-surface-2)]/50 hover:text-[var(--app-text-heading)] transition-all duration-200"
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <rect x="2" y="3" width="20" height="14" rx="2" ry="2" /><line x1="8" y1="21" x2="16" y2="21" /><line x1="12" y1="17" x2="12" y2="21" />
            </svg>
            {tn("archive")}
          </Link>
          <Link
            href="/settings"
            className="flex items-center gap-3 px-4 py-3 rounded-lg text-sm font-medium text-[var(--app-text-nav)] hover:bg-[var(--app-surface-2)]/50 hover:text-[var(--app-text-heading)] transition-all duration-200"
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 012.83-2.83l.06.06A1.65 1.65 0 009 4.68a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 2.83l-.06.06A1.65 1.65 0 0019.4 9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z" />
            </svg>
            {tn("settings")}
          </Link>
        </nav>

        {/* Compact followed users */}
        {followedUsers.length > 0 && (
          <div className="px-5 pb-4">
            <p className="mb-2 text-[10px] font-bold uppercase tracking-widest text-[var(--app-text-muted)]">
              {t("followedUsers")}
            </p>
            <div className="flex flex-wrap gap-1.5">
              {followedUsers.map((u) => (
                <Link
                  key={u.id}
                  href={`/@${u.username}`}
                  title={u.name}
                  className="flex h-7 w-7 items-center justify-center rounded-full bg-[var(--app-surface-2)] text-xs font-semibold text-[var(--app-text-heading)] hover:ring-2 hover:ring-[var(--app-accent)] transition-all"
                >
                  {u.initial}
                </Link>
              ))}
            </div>
          </div>
        )}

        {/* Fan pages */}
        {fanPages.length > 0 && (
          <div className="px-5 pb-4">
            <p className="mb-2 text-[10px] font-bold uppercase tracking-widest text-[var(--app-text-muted)]">
              {t("fanPages")}
            </p>
            <ul className="space-y-1">
              {fanPages.map((p) => (
                <li key={p.id}>
                  <Link
                    href={p.slug ? `/p/${p.slug}` : `/@${p.ownerUsername}`}
                    className="flex items-center justify-between rounded-lg px-2 py-1.5 text-xs text-[var(--app-text-nav)] hover:bg-[var(--app-surface-2)] hover:text-[var(--app-text-heading)] transition-colors"
                  >
                    <span className="truncate">{p.icon} {p.name}</span>
                    <span className="ml-1 shrink-0 text-[var(--app-text-muted)]">{p.count}</span>
                  </Link>
                </li>
              ))}
            </ul>
          </div>
        )}

        {/* Bottom CTA */}
        <div className="p-4 border-t border-[var(--app-border-inner)]">
          <Link
            href="/compose"
            className="w-full py-3 bg-[var(--app-accent)] text-white rounded-lg font-bold text-sm text-center block hover:opacity-90 active:scale-95 transition-all"
          >
            New Post
          </Link>
          <div className="mt-4 flex flex-col gap-2">
            <Link href="/support" className="flex items-center gap-2 text-xs text-[var(--app-text-muted)] px-2 hover:underline">
              Support
            </Link>
            <Link href="/terms" className="flex items-center gap-2 text-xs text-[var(--app-text-muted)] px-2 hover:underline">
              Terms
            </Link>
          </div>
        </div>
      </aside>

      {/* ── Fixed Right Sidebar ── */}
      <aside className="fixed right-0 top-0 pt-16 h-screen w-80 bg-[var(--app-bg)] border-l border-[var(--app-border-inner)] overflow-y-auto hidden xl:block" style={{ scrollbarWidth: "none" }}>
        <div className="px-6 py-6 space-y-4">
          {/* Elevate Your Trust */}
          <div className="bg-[var(--app-surface-2)] rounded-2xl p-6 border-b-4 border-[var(--app-secondary-border)]">
            <h3 className="font-headline text-xl font-bold text-[var(--app-text-heading)] mb-2">Elevate Your Trust</h3>
            <p className="text-[11px] text-[var(--app-text-muted)] mb-5 leading-relaxed">
              Certified members receive priority feed placement and access to restricted intelligence layers.
            </p>
            <div className="space-y-3">
              <div className="flex items-center justify-between text-[10px] font-bold uppercase tracking-wider">
                <span className="text-[var(--app-text-muted)]">Identity Completeness</span>
                <span className="text-[var(--app-secondary)]">65%</span>
              </div>
              <div className="w-full h-1 bg-[var(--app-surface-4)] rounded-full overflow-hidden">
                <div className="h-full w-[65%] bg-[var(--app-secondary)] rounded-full" />
              </div>
            </div>
            <Link
              href="/settings"
              className="block w-full mt-5 py-3 bg-[var(--app-text-heading)] text-[var(--app-bg)] rounded-xl font-bold text-[10px] uppercase tracking-widest text-center hover:opacity-90 transition-opacity"
            >
              Resume Certification
            </Link>
          </div>

          {/* Network Health */}
          <div className="p-5 bg-[var(--app-surface-3)] rounded-2xl border border-[var(--app-border-inner)]">
            <p className="text-[9px] text-[var(--app-text-muted)] uppercase tracking-widest font-bold mb-3">Network Health</p>
            <p className="font-headline text-2xl font-bold text-[var(--app-text-heading)]">99.98%</p>
            <p className="text-[10px] text-[var(--app-secondary)] mt-1">Nominal performance</p>
          </div>

          {/* Active Validators */}
          <div className="p-5 bg-[var(--app-surface-3)] rounded-2xl border border-[var(--app-border-inner)]">
            <p className="text-[9px] text-[var(--app-text-muted)] uppercase tracking-widest font-bold mb-3">Active Validators</p>
            <p className="font-headline text-2xl font-bold text-[var(--app-text-heading)]">14,202</p>
            <p className="text-[10px] text-[var(--app-accent)] mt-1">+12 in the last hour</p>
          </div>

          {/* Trending Intelligence */}
          <div className="p-5 bg-[var(--app-surface-3)] rounded-2xl border border-[var(--app-border-inner)]">
            <p className="text-[9px] text-[var(--app-text-muted)] uppercase tracking-widest font-bold mb-4">Trending Intelligence</p>
            <ul className="space-y-4">
              {trendingTags.map(({ tag, vouches }) => (
                <li key={tag} className="group cursor-pointer">
                  <p className="text-xs font-bold text-[var(--app-text-heading)] group-hover:text-[var(--app-accent)] transition-colors">{tag}</p>
                  <p className="text-[10px] text-[var(--app-text-muted)]">{vouches} vouches</p>
                </li>
              ))}
            </ul>
          </div>

          {/* Footer links */}
          <div className="flex flex-wrap gap-4 px-1 opacity-30 hover:opacity-100 transition-opacity pb-4">
            {["API", "NODES", "SECURITY", "TRANSPARENCY"].map((item) => (
              <a key={item} href="#" className="text-[9px] font-bold uppercase tracking-tighter text-[var(--app-text-heading)]">
                {item}
              </a>
            ))}
          </div>
        </div>
      </aside>

      {/* ── Main Content ── */}
      <main className="md:ml-64 xl:mr-80 pt-20 pb-24 md:pb-10 px-4 sm:px-6 md:px-10 lg:px-16">
        {children}
      </main>

      {/* ── Floating compose button (mobile, above bottom nav) ── */}
      <Link
        href="/compose"
        className="fixed bottom-20 right-5 w-14 h-14 bg-[var(--app-accent)] text-white rounded-2xl shadow-2xl flex items-center justify-center hover:scale-110 active:scale-95 transition-all z-40 md:hidden"
        aria-label="New post"
      >
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" />
        </svg>
      </Link>

      {/* ── Mobile Bottom Navigation ── */}
      <nav className="fixed bottom-0 left-0 right-0 z-50 md:hidden bg-[var(--app-header)] border-t border-[var(--app-border-inner)] flex items-stretch h-16"
        style={{ paddingBottom: "env(safe-area-inset-bottom)" }}>
        {/* Feed */}
        <Link href="/" className={`flex flex-1 flex-col items-center justify-center gap-1 text-[10px] font-bold uppercase tracking-widest transition-colors ${
          !activeTab || activeTab === "feed"
            ? "text-[var(--app-accent)]"
            : "text-[var(--app-text-muted)] hover:text-[var(--app-text-heading)]"
        }`}>
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={!activeTab || activeTab === "feed" ? "2.2" : "1.6"} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          Feed
        </Link>

        {/* Notes */}
        <Link href="/notes" className={`flex flex-1 flex-col items-center justify-center gap-1 text-[10px] font-bold uppercase tracking-widest transition-colors ${
          activeTab === "notes"
            ? "text-[var(--app-accent)]"
            : "text-[var(--app-text-muted)] hover:text-[var(--app-text-heading)]"
        }`}>
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={activeTab === "notes" ? "2.2" : "1.6"} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2" /><circle cx="9" cy="7" r="4" /><path d="M23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75" />
          </svg>
          {tn("notes")}
        </Link>

        {/* Archive */}
        <Link href="/settings" className="flex flex-1 flex-col items-center justify-center gap-1 text-[10px] font-bold uppercase tracking-widest text-[var(--app-text-muted)] hover:text-[var(--app-text-heading)] transition-colors">
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <rect x="2" y="3" width="20" height="14" rx="2" ry="2" /><line x1="8" y1="21" x2="16" y2="21" /><line x1="12" y1="17" x2="12" y2="21" />
          </svg>
          {tn("archive")}
        </Link>

        {/* Settings */}
        <Link href="/settings" className="flex flex-1 flex-col items-center justify-center gap-1 text-[10px] font-bold uppercase tracking-widest text-[var(--app-text-muted)] hover:text-[var(--app-text-heading)] transition-colors">
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 012.83-2.83l.06.06A1.65 1.65 0 009 4.68a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 2.83l-.06.06A1.65 1.65 0 0019.4 9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z" />
          </svg>
          {tn("settings")}
        </Link>
      </nav>
    </div>
  );
}
