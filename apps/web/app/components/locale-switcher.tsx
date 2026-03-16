"use client";

import { useRouter } from "next/navigation";
import { useTransition } from "react";
import { locales, type Locale } from "@/i18n/config";

const LABELS: Record<Locale, string> = {
  en: "EN",
  "zh-TW": "中文",
  ja: "日本語",
  ko: "한국어",
  fr: "FR",
};

export function LocaleSwitcher({ current }: { current: Locale }) {
  const router = useRouter();
  const [isPending, startTransition] = useTransition();

  function switchTo(locale: Locale) {
    document.cookie = `locale=${locale}; path=/; max-age=31536000`;
    startTransition(() => {
      router.refresh();
    });
  }

  return (
    <div className="flex items-center gap-1 text-xs">
      {locales.map((locale) => (
        <button
          key={locale}
          type="button"
          disabled={isPending || locale === current}
          onClick={() => switchTo(locale)}
          className={`rounded px-1.5 py-0.5 transition-colors ${
            locale === current
              ? "text-[#f09a45] font-semibold cursor-default"
              : "text-[#7a8090] hover:text-[#c8cdd8]"
          }`}
        >
          {LABELS[locale]}
        </button>
      ))}
    </div>
  );
}
