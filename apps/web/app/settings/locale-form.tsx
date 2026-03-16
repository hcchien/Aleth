"use client";

import { useRouter } from "next/navigation";
import { useTransition } from "react";
import { locales, type Locale } from "@/i18n/config";

const LOCALE_OPTIONS: { locale: Locale; native: string; english: string }[] = [
  { locale: "en",    native: "English",    english: "English" },
  { locale: "zh-TW", native: "繁體中文",   english: "Traditional Chinese" },
  { locale: "ja",    native: "日本語",     english: "Japanese" },
  { locale: "ko",    native: "한국어",     english: "Korean" },
  { locale: "fr",    native: "Français",   english: "French" },
];

export function LocaleForm({ current }: { current: Locale }) {
  const router = useRouter();
  const [isPending, startTransition] = useTransition();

  function switchTo(locale: Locale) {
    document.cookie = `locale=${locale}; path=/; max-age=31536000`;
    startTransition(() => {
      router.refresh();
    });
  }

  return (
    <div className="flex flex-col gap-2">
      {LOCALE_OPTIONS.map(({ locale, native, english }) => {
        const isActive = locale === current;
        return (
          <button
            key={locale}
            type="button"
            disabled={isPending || isActive}
            onClick={() => switchTo(locale)}
            className={`flex items-center gap-3 rounded-lg border px-4 py-3 text-left transition-colors ${
              isActive
                ? "border-[var(--app-accent)]/40 bg-[var(--app-accent)]/10 text-[var(--app-accent)] cursor-default"
                : "border-[var(--app-border-2)] text-[var(--app-text-secondary)] hover:border-[var(--app-border-hover)] hover:text-[var(--app-text-nav)] disabled:opacity-50"
            }`}
          >
            <span className="text-base font-medium">{native}</span>
            <span className="text-xs text-[var(--app-text-muted)]">{english}</span>
            {isActive && <span className="ml-auto text-sm">✓</span>}
          </button>
        );
      })}
    </div>
  );
}
