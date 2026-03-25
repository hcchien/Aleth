"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useAuth } from "@/lib/auth-context";
import { gqlClient } from "@/lib/gql-client";
import { PhoneVerificationForm } from "./phone-form";
import { SocialVerifyButton } from "./social-verify-button";

// ─── GraphQL ──────────────────────────────────────────────────────────────────

const MY_REPUTATION_QUERY = `
  query MyReputation {
    myReputation {
      stamps { provider score maxScore verifiedAt expiresAt isValid }
      totalScore threshold isL2
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

// ─── Provider config ──────────────────────────────────────────────────────────

const PROVIDERS = [
  { id: "phone",     label: "Phone Number",  max: 5, available: true },
  { id: "twitter",   label: "Twitter / X",   max: 4, available: true },
  { id: "instagram", label: "Instagram",     max: 5, available: true },
  { id: "facebook",  label: "Facebook",      max: 5, available: true },
  { id: "linkedin",  label: "LinkedIn",      max: 3, available: true },
] as const;

// ─── Icons ────────────────────────────────────────────────────────────────────

function IconPhone() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <rect x="5" y="2" width="14" height="20" rx="2" /><line x1="12" y1="18" x2="12.01" y2="18" strokeWidth="2.5" />
    </svg>
  );
}

function IconSocial() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="18" cy="5" r="3" /><circle cx="6" cy="12" r="3" /><circle cx="18" cy="19" r="3" />
      <line x1="8.59" y1="13.51" x2="15.42" y2="17.49" /><line x1="15.41" y1="6.51" x2="8.59" y2="10.49" />
    </svg>
  );
}

function IconCheck() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
      <path d="M20 6L9 17l-5-5" />
    </svg>
  );
}

// ─── Score ring ───────────────────────────────────────────────────────────────

function ScoreRing({ score, threshold, isL2 }: { score: number; threshold: number; isL2: boolean }) {
  const r = 36;
  const circ = 2 * Math.PI * r;
  const dash = circ * Math.min(1, score / threshold);

  return (
    <div className="relative flex items-center justify-center">
      <svg width="96" height="96" viewBox="0 0 96 96" className="rotate-[-90deg]">
        <circle cx="48" cy="48" r={r} fill="none" stroke="var(--app-border-inner)" strokeWidth="6" />
        <circle
          cx="48" cy="48" r={r} fill="none"
          stroke={isL2 ? "var(--app-verified)" : "var(--app-accent)"}
          strokeWidth="6"
          strokeDasharray={`${dash} ${circ}`}
          strokeLinecap="round"
          className="transition-all duration-700"
        />
      </svg>
      <div className="absolute flex flex-col items-center">
        <span className="text-xl font-bold text-[var(--app-text-heading)] leading-none tabular-nums">{score}</span>
        <span className="text-[10px] text-[var(--app-text-dim)] mt-0.5">/ {threshold}</span>
      </div>
    </div>
  );
}

// ─── Stamp card ───────────────────────────────────────────────────────────────

function StampCard({
  provider,
  stamp,
  expanded,
  onExpand,
  onVerified,
}: {
  provider: (typeof PROVIDERS)[number];
  stamp: ReputationStamp | undefined;
  expanded: boolean;
  onExpand: () => void;
  onVerified: () => void;
}) {
  const isVerified = stamp?.isValid ?? false;
  const earned = stamp?.score ?? 0;
  const barPct = Math.round((earned / provider.max) * 100);
  const Icon = provider.id === "phone" ? IconPhone : IconSocial;

  return (
    <div
      className={`rounded-xl border transition-colors ${
        isVerified
          ? "border-[var(--app-verified-border)] bg-[var(--app-verified-bg)]"
          : expanded
          ? "border-[var(--app-accent-border)] bg-[var(--app-surface-2)]"
          : "border-[var(--app-border-inner)] bg-[var(--app-surface-3)]"
      }`}
    >
      <button
        type="button"
        onClick={onExpand}
        disabled={isVerified || !provider.available}
        className="flex w-full items-center gap-3 p-4 text-left disabled:cursor-default"
      >
        {/* Status icon */}
        <div className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-lg ${isVerified ? "bg-[var(--app-verified)] text-white" : "bg-[var(--app-surface-4)] text-[var(--app-text-muted)]"}`}>
          {isVerified ? <IconCheck /> : <Icon />}
        </div>

        {/* Label + bar */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center justify-between mb-1.5">
            <span className="text-sm font-bold text-[var(--app-text-heading)]">{provider.label}</span>
            <span className={`text-[11px] font-mono tabular-nums ${isVerified ? "text-[var(--app-verified)]" : "text-[var(--app-text-dim)]"}`}>
              {earned} / {provider.max} pts
            </span>
          </div>
          <div className="h-1 w-full rounded-full bg-[var(--app-border-inner)] overflow-hidden">
            <div
              className={`h-full rounded-full transition-all duration-500 ${isVerified ? "bg-[var(--app-verified)]" : "bg-[var(--app-accent)]"}`}
              style={{ width: `${barPct}%` }}
            />
          </div>
        </div>

        {/* Right label */}
        <div className="shrink-0 ml-1 w-10 text-right">
          {isVerified ? (
            <span className="text-[10px] font-bold uppercase tracking-widest text-[var(--app-verified)]">Done</span>
          ) : !provider.available ? (
            <span className="text-[10px] font-bold uppercase tracking-widest text-[var(--app-text-dim)]">Soon</span>
          ) : (
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" className={`ml-auto transition-transform ${expanded ? "rotate-90" : ""} text-[var(--app-text-dim)]`}>
              <path d="M9 18l6-6-6-6" />
            </svg>
          )}
        </div>
      </button>

      {/* Expanded form */}
      {expanded && provider.id === "phone" && (
        <div className="border-t border-[var(--app-border-inner)] px-4 pb-4 pt-3">
          <PhoneVerificationForm alreadyVerified={false} onVerified={onVerified} />
        </div>
      )}
      {expanded && provider.id !== "phone" && (
        <div className="border-t border-[var(--app-border-inner)] px-4 pb-4 pt-3">
          <p className="mb-3 text-xs text-[var(--app-text-secondary)] leading-relaxed">
            Connect your {provider.label} account to earn up to {provider.max} reputation points.
            You will be redirected to {provider.label} to authorise and then returned here.
          </p>
          <SocialVerifyButton provider={provider.id} onVerified={onVerified} />
        </div>
      )}
    </div>
  );
}

// ─── Main component ───────────────────────────────────────────────────────────

export function ReputationClient() {
  const t = useTranslations("reputation");
  const { user } = useAuth();

  const [status, setStatus] = useState<ReputationStatus | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [expandedProvider, setExpandedProvider] = useState<string | null>(null);

  const fetchReputation = useCallback(async () => {
    if (!user) return;
    try {
      const data = await gqlClient<{ myReputation: ReputationStatus }>(MY_REPUTATION_QUERY, {});
      setStatus(data.myReputation);
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : "Failed to load reputation");
    }
  }, [user]);

  useEffect(() => { void fetchReputation(); }, [fetchReputation]);

  const stampByProvider = status
    ? Object.fromEntries(status.stamps.map((s) => [s.provider, s]))
    : {};
  const totalMax = PROVIDERS.reduce((s, p) => s + p.max, 0);

  return (
    <div className="mx-auto max-w-xl">
      {/* Header */}
      <div className="mb-8 flex items-center gap-4">
        <Link
          href="/settings"
          className="text-[var(--app-text-muted)] hover:text-[var(--app-text-secondary)] transition-colors"
          aria-label="Back to settings"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
            <path d="M15 18l-6-6 6-6" />
          </svg>
        </Link>
        <div>
          <p className="text-[10px] font-bold uppercase tracking-widest text-[var(--app-text-muted)]">Settings</p>
          <h1
            className="text-2xl font-bold text-[var(--app-text-heading)]"
            style={{ fontFamily: "var(--font-newsreader)" }}
          >
            {t("title")}
          </h1>
        </div>
      </div>

      {loadError && (
        <div className="mb-6 rounded-xl border border-red-500/20 bg-red-500/5 p-4 text-sm text-red-400">
          {loadError}
        </div>
      )}

      {/* Score summary */}
      <div className="mb-4 rounded-2xl border border-[var(--app-border-inner)] bg-[var(--app-surface-3)] p-5">
        <div className="flex items-center gap-6">
          <ScoreRing
            score={status?.totalScore ?? 0}
            threshold={status?.threshold ?? 10}
            isL2={status?.isL2 ?? false}
          />
          <div className="flex-1">
            <p className={`text-[10px] font-bold uppercase tracking-widest mb-1 ${status?.isL2 ? "text-[var(--app-verified)]" : "text-[var(--app-text-muted)]"}`}>
              {status?.isL2 ? "✓ Level 2 Achieved" : `Level ${user?.trustLevel ?? 0}`}
            </p>
            <p className="text-sm text-[var(--app-text-secondary)] mb-3">
              {status?.isL2
                ? "Your identity is fully verified."
                : `Earn ${(status?.threshold ?? 10) - (status?.totalScore ?? 0)} more points to reach Level 2.`}
            </p>
            {/* Dot legend */}
            <div className="flex flex-wrap gap-x-4 gap-y-1">
              {PROVIDERS.map((p) => {
                const s = stampByProvider[p.id];
                return (
                  <div key={p.id} className="flex items-center gap-1">
                    <div className={`h-1.5 w-1.5 rounded-full ${(s as ReputationStamp | undefined)?.isValid ? "bg-[var(--app-verified)]" : "bg-[var(--app-border-inner)]"}`} />
                    <span className="text-[10px] text-[var(--app-text-dim)]">{p.label.split(" ")[0]}</span>
                  </div>
                );
              })}
            </div>
          </div>
        </div>
      </div>

      {/* Stamp cards */}
      <p className="text-[10px] font-bold uppercase tracking-widest text-[var(--app-text-muted)] mb-3">
        Verification Methods · {status?.totalScore ?? 0} / {totalMax} pts
      </p>
      <div className="space-y-2">
        {PROVIDERS.map((p) => (
          <StampCard
            key={p.id}
            provider={p}
            stamp={stampByProvider[p.id] as ReputationStamp | undefined}
            expanded={expandedProvider === p.id}
            onExpand={() => {
              if (!p.available || (stampByProvider[p.id] as ReputationStamp | undefined)?.isValid) return;
              setExpandedProvider((prev) => (prev === p.id ? null : p.id));
            }}
            onVerified={() => {
              setExpandedProvider(null);
              void fetchReputation();
            }}
          />
        ))}
      </div>

      <p className="mt-6 text-center text-[11px] text-[var(--app-text-dim)]">
        Social stamps expire after 30 days and can be re-verified at any time.
      </p>
    </div>
  );
}
