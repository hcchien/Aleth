"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useAuth } from "@/lib/auth-context";
import { gqlClient } from "@/lib/gql-client";

// ─── GraphQL ──────────────────────────────────────────────────────────────────

const REPUTATION_QUERY = `
  query MyReputation {
    myReputation {
      stamps { provider score maxScore isValid expiresAt }
      totalScore threshold isL2
    }
  }
`;

// ─── Types ────────────────────────────────────────────────────────────────────

interface Stamp {
  provider: string;
  score: number;
  maxScore: number;
  isValid: boolean;
  expiresAt: string | null;
}

interface Reputation {
  stamps: Stamp[];
  totalScore: number;
  threshold: number;
  isL2: boolean;
}

// ─── Provider config ──────────────────────────────────────────────────────────

const PROVIDERS = [
  { id: "phone",     label: "Phone",     max: 5 },
  { id: "instagram", label: "Instagram", max: 5 },
  { id: "facebook",  label: "Facebook",  max: 5 },
  { id: "twitter",   label: "Twitter/X", max: 4 },
  { id: "linkedin",  label: "LinkedIn",  max: 3 },
];

// ─── Trust level config ───────────────────────────────────────────────────────

const LEVEL_CONFIG = [
  {
    level: 0, label: "Observer",
    color: "text-[var(--app-text-muted)]",
    bg: "bg-[var(--app-surface-4)]",
    border: "border-[var(--app-border-inner)]",
    desc: "Read-only access. Cannot post or react.",
  },
  {
    level: 1, label: "Contributor",
    color: "text-[var(--app-accent)]",
    bg: "bg-[var(--app-accent-bg)]",
    border: "border-[var(--app-accent-border)]",
    desc: "Can post, comment, and react. Verified email.",
  },
  {
    level: 2, label: "Verified Member",
    color: "text-[var(--app-verified)]",
    bg: "bg-[var(--app-verified-bg)]",
    border: "border-[var(--app-verified-border)]",
    desc: "Full network access. Identity cryptographically attested.",
  },
];

// ─── Icons ────────────────────────────────────────────────────────────────────

function IconShield({ verified }: { verified: boolean }) {
  return (
    <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 2L21 5.5V11.5C21 16.5 17 20.5 12 22C7 20.5 3 16.5 3 11.5V5.5Z" />
      {verified && <path d="M8.5 12l2.5 2.5 4.5-5" strokeWidth="2" />}
    </svg>
  );
}

function IconCopy() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <rect x="9" y="9" width="13" height="13" rx="2" />
      <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1" />
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

// ─── DID display ──────────────────────────────────────────────────────────────

function DidCard({ did }: { did: string }) {
  const [copied, setCopied] = useState(false);

  function copy() {
    void navigator.clipboard.writeText(did).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }

  const short = did.length > 44 ? `${did.slice(0, 22)}…${did.slice(-14)}` : did;

  return (
    <div className="rounded-2xl border border-[var(--app-border-inner)] bg-[var(--app-surface-3)] p-5">
      <p className="text-[10px] font-bold uppercase tracking-widest text-[var(--app-text-muted)] mb-3">
        Decentralized Identifier
      </p>
      <div className="flex items-center gap-3 mb-3">
        <code className="flex-1 font-mono text-xs text-[var(--app-text-secondary)] break-all leading-relaxed">
          {short}
        </code>
        <button
          type="button"
          onClick={copy}
          className="shrink-0 flex items-center gap-1.5 rounded-lg border border-[var(--app-border-inner)] bg-[var(--app-surface-4)] px-3 py-1.5 text-[11px] font-bold text-[var(--app-text-muted)] hover:border-[var(--app-accent-border)] hover:text-[var(--app-accent)] transition-colors"
        >
          {copied ? <IconCheck /> : <IconCopy />}
          <span>{copied ? "Copied" : "Copy"}</span>
        </button>
      </div>
      <p className="text-[11px] text-[var(--app-text-dim)] leading-relaxed">
        A globally unique, cryptographically anchored identifier you fully control. Used to sign all content you publish on Aleth.
      </p>
    </div>
  );
}

// ─── Trust level badge ────────────────────────────────────────────────────────

function TrustBadge({ level, isL2 }: { level: number; isL2: boolean }) {
  const cfg = LEVEL_CONFIG[Math.min(level, 2)];
  return (
    <div className={`flex items-center gap-4 rounded-2xl border p-5 ${cfg.border} ${cfg.bg}`}>
      <div className={cfg.color}>
        <IconShield verified={isL2} />
      </div>
      <div>
        <div className="flex items-center gap-2 mb-0.5">
          <span className={`text-[10px] font-bold uppercase tracking-widest ${cfg.color}`}>
            Level {level}
          </span>
          <span className={`text-sm font-bold ${cfg.color}`}>{cfg.label}</span>
        </div>
        <p className="text-xs text-[var(--app-text-secondary)]">{cfg.desc}</p>
      </div>
    </div>
  );
}

// ─── Verification progress ────────────────────────────────────────────────────

function VerificationProgress({ rep }: { rep: Reputation }) {
  const pct = Math.min(100, Math.round((rep.totalScore / rep.threshold) * 100));
  const stampMap = Object.fromEntries(rep.stamps.map((s) => [s.provider, s]));

  return (
    <div className="rounded-2xl border border-[var(--app-border-inner)] bg-[var(--app-surface-3)] p-5">
      {/* Header */}
      <div className="flex items-center justify-between mb-3">
        <p className="text-[10px] font-bold uppercase tracking-widest text-[var(--app-text-muted)]">
          Identity Score
        </p>
        <span className={`text-[11px] font-mono font-bold tabular-nums ${rep.isL2 ? "text-[var(--app-verified)]" : "text-[var(--app-text-secondary)]"}`}>
          {rep.totalScore} / {rep.threshold} pts
        </span>
      </div>

      {/* Progress bar */}
      <div className="h-1.5 w-full rounded-full bg-[var(--app-border-inner)] overflow-hidden mb-4">
        <div
          className={`h-full rounded-full transition-all duration-700 ${rep.isL2 ? "bg-[var(--app-verified)]" : "bg-[var(--app-accent)]"}`}
          style={{ width: `${pct}%` }}
        />
      </div>

      {/* Provider rows */}
      <div className="space-y-2.5 mb-4">
        {PROVIDERS.map((p) => {
          const stamp = stampMap[p.id];
          const done = stamp?.isValid ?? false;
          const earned = stamp?.score ?? 0;
          return (
            <div key={p.id} className="flex items-center gap-3">
              <div className={`h-2 w-2 rounded-full shrink-0 transition-colors ${done ? "bg-[var(--app-verified)]" : "bg-[var(--app-border-inner)]"}`} />
              <span className="flex-1 text-sm text-[var(--app-text)]">{p.label}</span>
              <div className="w-20 h-1 rounded-full bg-[var(--app-border-inner)] overflow-hidden">
                <div
                  className={`h-full rounded-full ${done ? "bg-[var(--app-verified)]" : "bg-[var(--app-accent)]"}`}
                  style={{ width: `${Math.round((earned / p.max) * 100)}%` }}
                />
              </div>
              <span className={`text-[11px] font-mono w-8 text-right tabular-nums ${done ? "text-[var(--app-verified)]" : "text-[var(--app-text-dim)]"}`}>
                {earned}/{p.max}
              </span>
            </div>
          );
        })}
      </div>

      {/* CTA */}
      <div className="pt-4 border-t border-[var(--app-border-inner)]">
        <Link
          href="/settings/reputation"
          className="group flex items-center justify-between"
        >
          <span className="text-sm font-bold text-[var(--app-accent)]">
            {rep.isL2 ? "Manage verification" : "Start verifying"}
          </span>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="text-[var(--app-accent)] group-hover:translate-x-0.5 transition-transform">
            <path d="M5 12h14M12 5l7 7-7 7" />
          </svg>
        </Link>
      </div>
    </div>
  );
}

// ─── Explainer ────────────────────────────────────────────────────────────────

function IdentityExplainer() {
  return (
    <div className="rounded-2xl border border-[var(--app-border-inner)] bg-[var(--app-surface-3)] p-5">
      <p className="text-[10px] font-bold uppercase tracking-widest text-[var(--app-text-muted)] mb-3">
        How Aleth Identity Works
      </p>
      <div className="space-y-3 text-sm text-[var(--app-text-secondary)] leading-relaxed">
        <p>
          Every account on Aleth is assigned a <strong className="text-[var(--app-text)]">Decentralized Identifier (DID)</strong> at registration — a unique, cryptographically verifiable key you fully own.
        </p>
        <p>
          All posts you publish are <strong className="text-[var(--app-text)]">signed with your DID</strong>, making them tamper-evident and attributable even across federated servers.
        </p>
        <p>
          Linking external accounts accumulates <strong className="text-[var(--app-text)]">Identity Score</strong>. Reach 10 points to unlock <strong className="text-[var(--app-text)]">Level 2 — Verified Member</strong>.
        </p>
      </div>
    </div>
  );
}

// ─── Main client component ────────────────────────────────────────────────────

export function IdentityClient() {
  const { user, loading } = useAuth();
  const [rep, setRep] = useState<Reputation | null>(null);

  const fetchRep = useCallback(async () => {
    if (!user) return;
    try {
      const data = await gqlClient<{ myReputation: Reputation }>(REPUTATION_QUERY, {});
      setRep(data.myReputation);
    } catch {
      // silently degrade
    }
  }, [user]);

  useEffect(() => { void fetchRep(); }, [fetchRep]);

  if (loading) {
    return (
      <div className="mx-auto max-w-xl">
        <div className="h-8 w-48 rounded-lg bg-[var(--app-surface-4)] animate-pulse mb-2" />
        <div className="h-6 w-32 rounded-lg bg-[var(--app-surface-4)] animate-pulse mb-8" />
        <div className="space-y-3">
          {[1, 2, 3].map((i) => (
            <div key={i} className="h-24 rounded-2xl bg-[var(--app-surface-4)] animate-pulse" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-xl">
      {/* Page heading */}
      <div className="mb-8">
        <p className="text-[10px] font-bold uppercase tracking-widest text-[var(--app-text-muted)] mb-1">
          Network Identity
        </p>
        <h1
          className="text-3xl font-bold text-[var(--app-text-heading)]"
          style={{ fontFamily: "var(--font-newsreader)" }}
        >
          Your Identity
        </h1>
      </div>

      {!user ? (
        <div className="rounded-2xl border border-[var(--app-border-inner)] bg-[var(--app-surface-3)] p-10 text-center">
          <p className="text-sm text-[var(--app-text-secondary)] mb-4">
            Sign in to view and manage your identity.
          </p>
          <Link
            href="/login"
            className="inline-flex items-center gap-2 rounded-xl bg-[var(--app-accent)] px-5 py-2 text-sm font-bold text-white hover:opacity-90 transition-opacity"
          >
            Sign in
          </Link>
        </div>
      ) : (
        <div className="space-y-3">
          <TrustBadge level={user.trustLevel} isL2={rep?.isL2 ?? false} />
          {user.did && <DidCard did={user.did} />}
          {rep && <VerificationProgress rep={rep} />}
          <IdentityExplainer />
          <div className="pt-1">
            <Link
              href="/settings"
              className="text-xs text-[var(--app-text-muted)] hover:text-[var(--app-text-secondary)] transition-colors"
            >
              ← Back to Settings
            </Link>
          </div>
        </div>
      )}
    </div>
  );
}
