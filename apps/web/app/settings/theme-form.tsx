"use client";

import { useRouter } from "next/navigation";
import { useTransition } from "react";
import { useTranslations } from "next-intl";

export type Theme = "dark" | "light";

const THEME_OPTIONS: { theme: Theme; icon: string }[] = [
  { theme: "dark",  icon: "◑" },
  { theme: "light", icon: "○" },
];

export function ThemeForm({ current }: { current: Theme }) {
  const t = useTranslations("settings");
  const router = useRouter();
  const [isPending, startTransition] = useTransition();

  function switchTo(theme: Theme) {
    document.cookie = `theme=${theme}; path=/; max-age=31536000`;
    startTransition(() => {
      router.refresh();
    });
  }

  return (
    <div className="flex flex-col gap-2">
      {THEME_OPTIONS.map(({ theme, icon }) => {
        const isActive = theme === current;
        const label = theme === "dark" ? t("themeDark") : t("themeLight");
        return (
          <button
            key={theme}
            type="button"
            disabled={isPending || isActive}
            onClick={() => switchTo(theme)}
            className={`flex items-center gap-3 rounded-lg border px-4 py-3 text-left transition-colors ${
              isActive
                ? "border-[var(--app-accent)]/40 bg-[var(--app-accent)]/10 text-[var(--app-accent)] cursor-default"
                : "border-[var(--app-border-2)] text-[var(--app-text-secondary)] hover:border-[var(--app-border-hover)] hover:text-[var(--app-text)] disabled:opacity-50"
            }`}
          >
            <span className="text-lg leading-none">{icon}</span>
            <span className="text-base font-medium">{label}</span>
            {isActive && <span className="ml-auto text-sm">✓</span>}
          </button>
        );
      })}
    </div>
  );
}
