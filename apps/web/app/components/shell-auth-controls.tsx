"use client";

import Link from "next/link";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useAuth } from "@/lib/auth-context";
import { LoginModal } from "./login-modal";
import { NotificationBell } from "./notification-bell";

export function ShellAuthControls() {
  const t = useTranslations("nav");
  const router = useRouter();
  const { user, loading, logout } = useAuth();
  const [showLogin, setShowLogin] = useState(false);

  if (loading) {
    return (
      <div className="h-9 w-20 rounded-sm border border-[var(--app-border-inner)] bg-[var(--app-input-bg)]" />
    );
  }

  if (!user) {
    return (
      <>
        <button
          type="button"
          onClick={() => setShowLogin(true)}
          className="rounded-sm border border-[var(--app-border-inner)] px-4 py-1.5 hover:bg-[var(--app-hover)]"
        >
          ◎ {t("signIn")}
        </button>
        <LoginModal isOpen={showLogin} onClose={() => setShowLogin(false)} />
      </>
    );
  }

  return (
    <div className="flex items-center gap-3">
      <NotificationBell />
      <Link
        href={`/@${user.username}`}
        className="max-w-40 truncate text-sm text-[var(--app-text-nav)] hover:text-[var(--app-text-heading)]"
        title={user.displayName ?? user.username}
      >
        {user.displayName ?? user.username}
      </Link>
      <Link
        href="/settings"
        className="rounded-sm border border-[var(--app-border-inner)] px-3 py-1.5 text-sm hover:bg-[var(--app-hover)]"
      >
        {t("settings")}
      </Link>
      <button
        type="button"
        onClick={() => {
          logout();
          router.push("/");
        }}
        className="rounded-sm border border-[var(--app-border-inner)] px-3 py-1.5 text-sm hover:bg-[var(--app-hover)]"
      >
        {t("signOut")}
      </button>
    </div>
  );
}
