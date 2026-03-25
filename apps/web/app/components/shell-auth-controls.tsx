"use client";

import Link from "next/link";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useAuth } from "@/lib/auth-context";
import { LoginModal } from "./login-modal";
import { NotificationBell } from "./notification-bell";
import { ThemeToggle } from "./theme-toggle";

// Verified badge shown in header when trust level >= 2
function VerifiedHeaderBadge({ level }: { level: number }) {
  if (level < 2) return null;
  return (
    <span className="inline-flex items-center gap-1 rounded-full border border-[var(--app-verified-border)] bg-[var(--app-verified-bg)] px-2.5 py-1 text-xs font-semibold text-[var(--app-verified)]">
      <svg width="9" height="9" viewBox="0 0 8 8" fill="currentColor" aria-hidden="true">
        <path d="M3.5 6.5L1 4l.7-.7 1.8 1.8 3.8-3.8.7.7z" />
      </svg>
      Verified
    </span>
  );
}

export function ShellAuthControls() {
  const t = useTranslations("nav");
  const router = useRouter();
  const { user, loading, logout } = useAuth();
  const [showLogin, setShowLogin] = useState(false);

  if (loading) {
    return (
      <div className="h-8 w-24 rounded-full border border-[var(--app-border-inner)] bg-[var(--app-input-bg)] animate-pulse" />
    );
  }

  if (!user) {
    return (
      <>
        <ThemeToggle />
        <button
          type="button"
          onClick={() => setShowLogin(true)}
          className="btn-primary text-xs px-4 py-2"
        >
          {t("signIn")}
        </button>
        <LoginModal isOpen={showLogin} onClose={() => setShowLogin(false)} />
      </>
    );
  }

  const initial = (user.displayName ?? user.username).slice(0, 1).toUpperCase();

  return (
    <div className="flex items-center gap-2">
      <VerifiedHeaderBadge level={user.trustLevel} />
      <ThemeToggle />
      <NotificationBell />
      {/* Avatar circle linking to profile */}
      <Link
        href={`/@${user.username}`}
        title={user.displayName ?? user.username}
        className="flex h-8 w-8 items-center justify-center rounded-full bg-[var(--app-accent-bg)] text-sm font-bold text-[var(--app-accent)] ring-2 ring-[var(--app-accent-border)] hover:ring-[var(--app-accent)] transition-all"
      >
        {initial}
      </Link>
      <button
        type="button"
        onClick={() => {
          logout();
          router.push("/");
        }}
        className="hidden text-xs text-[var(--app-text-muted)] hover:text-[var(--app-text-secondary)] transition-colors md:block"
        title={t("signOut")}
      >
        {t("signOut")}
      </button>
    </div>
  );
}
