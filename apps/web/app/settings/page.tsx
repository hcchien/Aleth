import { cookies } from "next/headers";
import { getLocale, getTranslations } from "next-intl/server";
import Link from "next/link";
import type { Locale } from "@/i18n/config";
import { LocaleForm } from "./locale-form";
import { ThemeForm } from "./theme-form";
import type { Theme } from "./theme-form";
import { FederationToggle } from "./federation-toggle";
import { RemoteFollows } from "./remote-follows";

export default async function SettingsPage() {
  const locale = (await getLocale()) as Locale;
  const t = await getTranslations("settings");
  const tn = await getTranslations("nav");
  const cookieStore = await cookies();
  const theme = (cookieStore.get("theme")?.value === "light" ? "light" : "dark") as Theme;

  return (
    <div className="min-h-screen bg-[var(--app-bg)] text-[var(--app-text)]">
      <div className="mx-auto max-w-2xl px-4 py-10 pb-20">
        {/* Breadcrumb */}
        <nav className="mb-8 flex items-center gap-2 text-sm text-[var(--app-text-muted)]">
          <Link href="/" className="hover:text-[var(--app-text-secondary)] transition-colors">
            {tn("feed")}
          </Link>
          <span>›</span>
          <span className="text-[var(--app-text-secondary)]">{t("title")}</span>
        </nav>

        <h1 className="mb-8 font-serif text-3xl text-[var(--app-text-heading)]">{t("title")}</h1>

        {/* Appearance */}
        <section className="mb-4 rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)] p-5">
          <h2 className="mb-1 text-sm font-semibold text-[var(--app-text-bright)]">{t("appearance")}</h2>
          <p className="mb-4 text-xs text-[var(--app-text-muted)]">{t("appearanceDesc")}</p>
          <ThemeForm current={theme} />
        </section>

        {/* Language */}
        <section className="mb-4 rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)] p-5">
          <h2 className="mb-1 text-sm font-semibold text-[var(--app-text-bright)]">{t("language")}</h2>
          <p className="mb-4 text-xs text-[var(--app-text-muted)]">{t("languageDesc")}</p>
          <LocaleForm current={locale} />
        </section>

        {/* Security */}
        <Link
          href="/settings/security"
          className="mb-4 flex items-center justify-between rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)] p-5 transition-colors hover:border-[var(--app-border-hover)] hover:bg-[var(--app-surface-hover)]"
        >
          <div>
            <h2 className="text-sm font-semibold text-[var(--app-text-bright)]">{t("security")}</h2>
            <p className="mt-0.5 text-xs text-[var(--app-text-muted)]">{t("securityDesc")}</p>
          </div>
          <span className="text-[var(--app-text-dim)]">›</span>
        </Link>

        {/* Reputation (L2) */}
        <Link
          href="/settings/reputation"
          className="mb-4 flex items-center justify-between rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)] p-5 transition-colors hover:border-[var(--app-border-hover)] hover:bg-[var(--app-surface-hover)]"
        >
          <div>
            <h2 className="text-sm font-semibold text-[var(--app-text-bright)]">{t("reputation")}</h2>
            <p className="mt-0.5 text-xs text-[var(--app-text-muted)]">{t("reputationDesc")}</p>
          </div>
          <span className="text-[var(--app-text-dim)]">›</span>
        </Link>

        {/* ActivityPub Federation */}
        <section className="mb-4 rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)] p-5">
          <h2 className="mb-1 text-sm font-semibold text-[var(--app-text-bright)]">{t("federation")}</h2>
          <p className="mb-4 text-xs text-[var(--app-text-muted)]">{t("federationDesc")}</p>
          <FederationToggle />
          <RemoteFollows />
        </section>

        {/* Board settings */}
        <Link
          href="/settings/board"
          className="mb-4 flex items-center justify-between rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)] p-5 transition-colors hover:border-[var(--app-border-hover)] hover:bg-[var(--app-surface-hover)]"
        >
          <div>
            <h2 className="text-sm font-semibold text-[var(--app-text-bright)]">{t("board")}</h2>
            <p className="mt-0.5 text-xs text-[var(--app-text-muted)]">{t("boardDesc")}</p>
          </div>
          <span className="text-[var(--app-text-dim)]">›</span>
        </Link>

        {/* Fan pages */}
        <Link
          href="/settings/pages"
          className="mb-4 flex items-center justify-between rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)] p-5 transition-colors hover:border-[var(--app-border-hover)] hover:bg-[var(--app-surface-hover)]"
        >
          <div>
            <h2 className="text-sm font-semibold text-[var(--app-text-bright)]">{t("pages")}</h2>
            <p className="mt-0.5 text-xs text-[var(--app-text-muted)]">{t("pagesDesc")}</p>
          </div>
          <span className="text-[var(--app-text-dim)]">›</span>
        </Link>

        {/* Article series */}
        <Link
          href="/settings/series"
          className="flex items-center justify-between rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)] p-5 transition-colors hover:border-[var(--app-border-hover)] hover:bg-[var(--app-surface-hover)]"
        >
          <div>
            <h2 className="text-sm font-semibold text-[var(--app-text-bright)]">{t("seriesTitle")}</h2>
            <p className="mt-0.5 text-xs text-[var(--app-text-muted)]">{t("seriesDesc")}</p>
          </div>
          <span className="text-[var(--app-text-dim)]">›</span>
        </Link>
      </div>
    </div>
  );
}
