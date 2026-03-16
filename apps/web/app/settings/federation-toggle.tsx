"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/lib/auth-context";
import { gqlClient } from "@/lib/gql-client";

const SET_AP_MUTATION = `
  mutation SetActivityPubEnabled($enabled: Boolean!) {
    setActivityPubEnabled(enabled: $enabled)
  }
`;

export function FederationToggle() {
  const t = useTranslations("settings");
  const { user, login } = useAuth();

  // Optimistic local state — initialised from user context
  const [enabled, setEnabled] = useState<boolean>(user?.apEnabled ?? true);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  // Sync if user context loads after mount
  if (user && user.apEnabled !== enabled && !saving) {
    setEnabled(user.apEnabled);
  }

  async function toggle() {
    if (!user || saving) return;
    const next = !enabled;
    setEnabled(next); // optimistic
    setSaving(true);
    setSaved(false);
    try {
      await gqlClient<{ setActivityPubEnabled: boolean }>(SET_AP_MUTATION, {
        enabled: next,
      });
      // Patch the user object in auth context so the rest of the app sees the change
      login(
        localStorage.getItem("accessToken") ?? "",
        localStorage.getItem("refreshToken") ?? "",
        { ...user, apEnabled: next }
      );
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
    } catch {
      setEnabled(!next); // revert on error
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="flex items-center justify-between">
      <p className="text-xs text-[var(--app-text-muted)]">
        {enabled ? t("federationEnabled") : t("federationDisabled")}
      </p>
      <div className="flex items-center gap-3">
        {saved && (
          <span className="text-xs text-green-500">{t("federationSaved")}</span>
        )}
        {saving && (
          <span className="text-xs text-[var(--app-text-muted)]">
            {t("federationSaving")}
          </span>
        )}
        {/* Toggle switch */}
        <button
          type="button"
          role="switch"
          aria-checked={enabled}
          disabled={saving || !user}
          onClick={toggle}
          className={[
            "relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent",
            "transition-colors duration-200 ease-in-out focus:outline-none disabled:opacity-40",
            enabled ? "bg-[var(--app-accent)]" : "bg-[var(--app-border-2)]",
          ].join(" ")}
        >
          <span
            className={[
              "pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow",
              "ring-0 transition duration-200 ease-in-out",
              enabled ? "translate-x-5" : "translate-x-0",
            ].join(" ")}
          />
        </button>
      </div>
    </div>
  );
}
