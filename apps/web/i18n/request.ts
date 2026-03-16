import { getRequestConfig } from "next-intl/server";
import { cookies, headers } from "next/headers";
import { locales, defaultLocale, type Locale } from "./config";

function detectLocaleFromAcceptLanguage(acceptLanguage: string | null): Locale {
  if (!acceptLanguage) return defaultLocale;
  // Parse "zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7"
  const preferred = acceptLanguage
    .split(",")
    .map((part) => {
      const [lang, q] = part.trim().split(";q=");
      return { lang: lang.trim(), q: q ? parseFloat(q) : 1.0 };
    })
    .sort((a, b) => b.q - a.q)
    .map((x) => x.lang);

  for (const lang of preferred) {
    // Exact match (e.g. "zh-TW")
    if ((locales as string[]).includes(lang)) return lang as Locale;
    // Prefix match: "zh" → "zh-TW", "en-US" → "en", "ja-JP" → "ja"
    const prefix = lang.split("-")[0];
    const match = locales.find((l) => l === prefix || l.startsWith(prefix + "-"));
    if (match) return match;
  }
  return defaultLocale;
}

export default getRequestConfig(async () => {
  const cookieStore = await cookies();
  const raw = cookieStore.get("locale")?.value;

  let locale: Locale;
  if (raw && (locales as string[]).includes(raw)) {
    // User has an explicit preference saved
    locale = raw as Locale;
  } else {
    // New user: detect from browser's Accept-Language header
    const headerStore = await headers();
    locale = detectLocaleFromAcceptLanguage(headerStore.get("accept-language"));
  }

  return {
    locale,
    messages: (await import(`../messages/${locale}.json`)).default,
  };
});
