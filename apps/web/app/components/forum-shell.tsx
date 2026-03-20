import Link from "next/link";
import { getTranslations } from "next-intl/server";
import { ShellAuthControls } from "./shell-auth-controls";
import { TrustBadge } from "./trust-badge";

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

export async function ForumShell({
  children,
  followedUsers = [],
  fanPages = [],
  activeTab,
}: {
  children: React.ReactNode;
  followedUsers?: FollowedUser[];
  fanPages?: FanPage[];
  activeTab?: "feed" | "notes";
}) {
  const t = await getTranslations("sidebar");
  const tn = await getTranslations("nav");

  return (
    <div className="min-h-screen bg-[var(--app-bg)] text-[var(--app-text)] md:grid md:grid-cols-[300px_1fr]">
      <aside className="border-r border-[var(--app-border)] bg-[var(--app-sidebar)] px-4 py-5 md:min-h-screen">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-3xl font-semibold tracking-wide">{t("heading")}</h2>
          <Link href="/settings" className="text-base text-[var(--app-text-secondary)] hover:text-[var(--app-text)] transition-colors" title={tn("settings")}>⚙</Link>
        </div>
        <Link
          href="/"
          className="mb-8 block w-full rounded-md border border-[var(--app-accent-border)] bg-[var(--app-accent-bg)] px-4 py-3 text-left text-sm text-[var(--app-accent)]"
        >
          {t("allFeed")}
        </Link>

        <section className="mb-10">
          <h3 className="mb-4 text-lg text-[var(--app-text-bright)]">{t("followedUsers")}</h3>
          <ul className="space-y-4">
            {followedUsers.map((u) => (
              <li key={u.name} className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <span className="flex h-8 w-8 items-center justify-center rounded-full bg-[var(--app-border-hover)] text-sm text-[var(--app-text-heading)]">
                    {u.initial}
                  </span>
                  <Link
                    href={`/@${u.username}`}
                    className="text-base text-[var(--app-text-heading)] hover:text-[var(--app-text)]"
                  >
                    {u.name}
                  </Link>
                </div>
                {u.trustLevel !== undefined ? (
                  <TrustBadge level={u.trustLevel} />
                ) : (
                  <span
                    className={`rounded-full border px-2 py-0.5 text-xs ${u.tierClass}`}
                  >
                    {u.tier}
                  </span>
                )}
              </li>
            ))}
            {followedUsers.length === 0 && (
              <li className="text-sm text-[var(--app-text-muted)]">{t("noData")}</li>
            )}
          </ul>
        </section>

        <section>
          <h3 className="mb-4 text-lg text-[var(--app-text-bright)]">{t("fanPages")}</h3>
          <ul className="space-y-4 text-base">
            {fanPages.map((p) => (
              <li
                key={p.name}
                className="flex items-center justify-between text-[var(--app-text-heading)]"
              >
                <Link href={p.slug ? `/p/${p.slug}` : `/@${p.ownerUsername}`} className="hover:text-[var(--app-text)]">
                  {p.icon}　{p.name}
                </Link>
                <span className="text-[var(--app-text-secondary)]">{p.count}</span>
              </li>
            ))}
            {fanPages.length === 0 && (
              <li className="text-sm text-[var(--app-text-muted)]">{t("noData")}</li>
            )}
          </ul>
        </section>

        <div className="mt-12 border-t border-[var(--app-border)] pt-5 text-sm text-[var(--app-text)]">
          {t("manageFollowing")}
        </div>
      </aside>

      <div className="min-h-screen bg-[var(--app-bg)]">
        <header className="sticky top-0 z-20 border-b border-[var(--app-border)] bg-[var(--app-header)] px-4 py-4 md:px-10">
          <div className="mx-auto flex max-w-5xl items-center justify-between">
            <div className="flex items-center gap-4">
              <Link
                href="/"
                className="text-2xl font-bold tracking-tight text-[var(--app-accent)] hover:opacity-85 transition-opacity"
                style={{ fontFamily: "var(--font-playfair), 'Playfair Display', Georgia, serif" }}
              >
                Aleth
              </Link>
              <nav className="flex items-center gap-1">
                <Link
                  href="/"
                  className={`rounded px-3 py-1.5 text-xs font-semibold tracking-widest uppercase transition-colors ${
                    !activeTab || activeTab === "feed"
                      ? "text-[var(--app-accent)] bg-[var(--app-accent-bg)]"
                      : "text-[var(--app-text-nav)] hover:text-[var(--app-text-heading)] hover:bg-[var(--app-hover)]"
                  }`}
                >
                  {tn("feed")}
                </Link>
                <Link
                  href="/notes"
                  className={`rounded px-3 py-1.5 text-xs font-semibold tracking-widest uppercase transition-colors ${
                    activeTab === "notes"
                      ? "text-[var(--app-accent)] bg-[var(--app-accent-bg)]"
                      : "text-[var(--app-text-nav)] hover:text-[var(--app-text-heading)] hover:bg-[var(--app-hover)]"
                  }`}
                >
                  {tn("notes")}
                </Link>
              </nav>
            </div>
            <div className="flex items-center gap-5 text-sm text-[var(--app-text-nav)]">
              <span className="flex items-center gap-2">
                <span className="inline-block h-2.5 w-2.5 rounded-full bg-emerald-400" />{" "}
                {tn("connected")}
              </span>
              <Link
                href="/settings"
                className="text-[var(--app-text-muted)] hover:text-[var(--app-text-nav)] transition-colors"
                title={tn("settings")}
              >
                ⚙
              </Link>
              <ShellAuthControls />
            </div>
          </div>
        </header>

        <main className="mx-auto max-w-4xl px-4 py-8 md:px-8">{children}</main>
      </div>
    </div>
  );
}
