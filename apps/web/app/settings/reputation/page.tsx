"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useAuth } from "@/lib/auth-context";
import { gqlClient } from "@/lib/gql-client";
import { PhoneVerificationForm } from "./phone-form";

// ─── GraphQL ──────────────────────────────────────────────────────────────────

const MY_REPUTATION_QUERY = `
  query MyReputation {
    myReputation {
      stamps {
        provider
        score
        maxScore
        verifiedAt
        expiresAt
        isValid
      }
      totalScore
      threshold
      isL2
    }
  }
`;

// ─── Types ────────────────────────────────────────────────────────────────────

interface ReputationStamp {
  provider: string;
  score: number;
  maxScore: number;
  verifiedAt: string;
  expiresAt: string | null;
  isValid: boolean;
}

interface ReputationStatus {
  stamps: ReputationStamp[];
  totalScore: number;
  threshold: number;
  isL2: boolean;
}

// ─── Provider config (ordered by priority) ────────────────────────────────────

const PROVIDER_ORDER = ["phone", "instagram", "facebook", "twitter", "linkedin"] as const;
type Provider = (typeof PROVIDER_ORDER)[number];

function providerLabel(provider: Provider, t: ReturnType<typeof useTranslations<"reputation">>) {
  const map: Record<Provider, string> = {
    phone: t("providerPhone"),
    instagram: t("providerInstagram"),
    facebook: t("providerFacebook"),
    twitter: t("providerTwitter"),
    linkedin: t("providerLinkedin"),
  };
  return map[provider] ?? provider;
}

const MAX_SCORES: Record<Provider, number> = {
  phone: 5,
  instagram: 5,
  facebook: 5,
  twitter: 4,
  linkedin: 3,
};

// ─── Progress bar ─────────────────────────────────────────────────────────────

function ScoreBar({ score, threshold }: { score: number; threshold: number }) {
  const pct = Math.min(100, Math.round((score / threshold) * 100));
  return (
    <div className="h-2 w-full overflow-hidden rounded-full bg-[var(--app-border)]">
      <div
        className={`h-full rounded-full transition-all duration-500 ${
          pct >= 100 ? "bg-lime-400" : "bg-[var(--app-accent)]"
        }`}
        style={{ width: `${pct}%` }}
      />
    </div>
  );
}

// ─── Stamp row ────────────────────────────────────────────────────────────────

function StampRow({
  provider,
  stamp,
  t,
}: {
  provider: Provider;
  stamp: ReputationStamp | undefined;
  t: ReturnType<typeof useTranslations<"reputation">>;
}) {
  const maxScore = MAX_SCORES[provider];
  const earned = stamp?.score ?? 0;
  const isValid = stamp?.isValid ?? false;

  return (
    <div className="flex items-center gap-3 py-2.5">
      {/* Status dot */}
      <div
        className={`h-2 w-2 shrink-0 rounded-full ${
          isValid ? "bg-lime-400" : stamp ? "bg-[var(--app-text-dim)]" : "bg-[var(--app-border)]"
        }`}
      />

      {/* Provider name */}
      <span className="flex-1 text-sm text-[var(--app-text)]">{providerLabel(provider, t)}</span>

      {/* Score badge */}
      {stamp && isValid ? (
        <span className="rounded-full bg-lime-400/10 px-2 py-0.5 text-xs font-medium text-lime-400">
          {t("earnedPts", { score: earned })}
        </span>
      ) : (
        <span className="text-xs text-[var(--app-text-dim)]">
          {t("maxPts", { max: maxScore })}
        </span>
      )}

      {/* Expiry */}
      {stamp && (
        <span className="hidden text-xs text-[var(--app-text-dim)] sm:block">
          {stamp.expiresAt
            ? t("expires", { date: new Date(stamp.expiresAt).toLocaleDateString() })
            : t("neverExpires")}
        </span>
      )}
    </div>
  );
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function ReputationPage() {
  const t = useTranslations("reputation");
  const ts = useTranslations("settings");
  const { user } = useAuth();

  const [status, setStatus] = useState<ReputationStatus | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);

  const fetchReputation = useCallback(async () => {
    if (!user) return;
    try {
      const data = await gqlClient<{ myReputation: ReputationStatus }>(MY_REPUTATION_QUERY, {});
      setStatus(data.myReputation);
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : "Failed to load reputation");
    }
  }, [user]);

  useEffect(() => {
    fetchReputation();
  }, [fetchReputation]);

  const stampByProvider = status
    ? Object.fromEntries(status.stamps.map((s) => [s.provider, s]))
    : {};

  const phoneStamp = stampByProvider["phone"] as ReputationStamp | undefined;

  return (
    <div className="min-h-screen bg-[var(--app-bg)] text-[var(--app-text)]">
      <div className="mx-auto max-w-2xl px-4 py-10 pb-20">
        {/* Back link */}
        <Link
          href="/settings"
          className="mb-8 inline-block text-sm text-[var(--app-text-muted)] hover:text-[var(--app-text-secondary)] transition-colors"
        >
          {t("backToSettings")}
        </Link>

        <h1 className="mb-1 font-serif text-3xl text-[var(--app-text-heading)]">{t("title")}</h1>
        <p className="mb-8 text-sm text-[var(--app-text-secondary)]">{t("subtitle")}</p>

        {/* Trust level + score summary */}
        <section className="mb-4 rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)] p-5">
          <div className="mb-3 flex items-center justify-between">
            <p className="text-sm text-[var(--app-text-secondary)]">
              {t("currentLevel", { level: user?.trustLevel ?? 0 })}
            </p>
            {status?.isL2 ? (
              <span className="text-sm font-semibold text-lime-400">{t("l2Achieved")}</span>
            ) : (
              <span className="text-xs text-[var(--app-text-dim)]">
                {status
                  ? t("score", { score: status.totalScore, threshold: status.threshold })
                  : "…"}
              </span>
            )}
          </div>
          {status && (
            <ScoreBar score={status.totalScore} threshold={status.threshold} />
          )}
        </section>

        {/* Stamp list */}
        <section className="mb-4 rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)] p-5">
          {loadError ? (
            <p className="text-sm text-red-400">{loadError}</p>
          ) : (
            <div className="divide-y divide-[var(--app-border)]">
              {PROVIDER_ORDER.map((provider) => (
                <StampRow
                  key={provider}
                  provider={provider}
                  stamp={stampByProvider[provider] as ReputationStamp | undefined}
                  t={t}
                />
              ))}
            </div>
          )}
        </section>

        {/* Phone verification form — only show if not yet verified */}
        {user && !phoneStamp?.isValid && (
          <section className="rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)] p-5">
            <PhoneVerificationForm
              alreadyVerified={!!phoneStamp?.isValid}
              onVerified={fetchReputation}
            />
          </section>
        )}

        {/* Placeholder for OAuth stamps */}
        {user && (
          <p className="mt-6 text-center text-xs text-[var(--app-text-dim)]">
            Instagram, Facebook, Twitter/X, and LinkedIn verification coming soon.
          </p>
        )}

        {!user && (
          <p className="text-sm text-[var(--app-text-muted)]">{ts("securityDesc")}</p>
        )}
      </div>
    </div>
  );
}
