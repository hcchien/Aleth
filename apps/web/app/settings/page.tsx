import { cookies } from "next/headers";
import { getLocale, getTranslations } from "next-intl/server";
import Link from "next/link";
import type { Locale } from "@/i18n/config";
import { LocaleForm } from "./locale-form";
import { ThemeForm } from "./theme-form";
import type { Theme } from "./theme-form";
import { FederationToggle } from "./federation-toggle";
import { RemoteFollows } from "./remote-follows";
import { ForumShell } from "@/app/components/forum-shell";

// ─── Section card ────────────────────────────────────────────────────────────

function SectionCard({ children }: { children: React.ReactNode }) {
  return (
    <div className="rounded-2xl border border-[var(--app-border-inner)] bg-[var(--app-surface-3)] p-5">
      {children}
    </div>
  );
}

function SectionLink({
  href,
  title,
  description,
  accent,
}: {
  href: string;
  title: string;
  description: string;
  accent?: boolean;
}) {
  return (
    <Link
      href={href}
      className={`group flex items-center justify-between rounded-2xl border p-5 transition-all ${
        accent
          ? "border-[var(--app-accent-border)] bg-[var(--app-accent-bg)] hover:border-[var(--app-accent)] hover:bg-[var(--app-accent-bg)]"
          : "border-[var(--app-border-inner)] bg-[var(--app-surface-3)] hover:border-[var(--app-accent-border)] hover:bg-[var(--app-surface-2)]"
      }`}
    >
      <div>
        <h2
          className={`text-sm font-bold tracking-tight ${
            accent ? "text-[var(--app-accent)]" : "text-[var(--app-text-heading)]"
          }`}
        >
          {title}
        </h2>
        <p className="mt-0.5 text-xs text-[var(--app-text-muted)]">{description}</p>
      </div>
      <svg
        width="16"
        height="16"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        className="shrink-0 text-[var(--app-text-dim)] group-hover:text-[var(--app-text-secondary)] transition-colors"
        aria-hidden="true"
      >
        <path d="M9 18l6-6-6-6" />
      </svg>
    </Link>
  );
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default async function SettingsPage() {
  const locale = (await getLocale()) as Locale;
  const t = await getTranslations("settings");
  const cookieStore = await cookies();
  const theme = (cookieStore.get("theme")?.value === "light" ? "light" : "dark") as Theme;

  return (
    <ForumShell>
      <div className="mx-auto max-w-xl">
        {/* Page heading */}
        <div className="mb-8">
          <p className="text-[10px] font-bold uppercase tracking-widest text-[var(--app-text-muted)] mb-1">
            Settings
          </p>
          <h1
            className="text-3xl font-bold text-[var(--app-text-heading)]"
            style={{ fontFamily: "var(--font-newsreader)" }}
          >
            {t("title")}
          </h1>
        </div>

        <div className="space-y-3">
          {/* Identity — primary CTA */}
          <SectionLink
            href="/identity"
            title="Identity & Verification"
            description="Manage your DID, trust level, and verification stamps."
            accent
          />

          {/* Reputation */}
          <SectionLink
            href="/settings/reputation"
            title={t("reputation")}
            description={t("reputationDesc")}
          />

          {/* Security */}
          <SectionLink
            href="/settings/security"
            title={t("security")}
            description={t("securityDesc")}
          />

          {/* Appearance */}
          <SectionCard>
            <h2 className="mb-1 text-sm font-bold tracking-tight text-[var(--app-text-heading)]">
              {t("appearance")}
            </h2>
            <p className="mb-4 text-xs text-[var(--app-text-muted)]">{t("appearanceDesc")}</p>
            <ThemeForm current={theme} />
          </SectionCard>

          {/* Language */}
          <SectionCard>
            <h2 className="mb-1 text-sm font-bold tracking-tight text-[var(--app-text-heading)]">
              {t("language")}
            </h2>
            <p className="mb-4 text-xs text-[var(--app-text-muted)]">{t("languageDesc")}</p>
            <LocaleForm current={locale} />
          </SectionCard>

          {/* ActivityPub Federation */}
          <SectionCard>
            <h2 className="mb-1 text-sm font-bold tracking-tight text-[var(--app-text-heading)]">
              {t("federation")}
            </h2>
            <p className="mb-4 text-xs text-[var(--app-text-muted)]">{t("federationDesc")}</p>
            <FederationToggle />
            <RemoteFollows />
          </SectionCard>

          {/* Board */}
          <SectionLink
            href="/settings/board"
            title={t("board")}
            description={t("boardDesc")}
          />

          {/* Fan pages */}
          <SectionLink
            href="/settings/pages"
            title={t("pages")}
            description={t("pagesDesc")}
          />

          {/* Series */}
          <SectionLink
            href="/settings/series"
            title={t("seriesTitle")}
            description={t("seriesDesc")}
          />
        </div>
      </div>
    </ForumShell>
  );
}
