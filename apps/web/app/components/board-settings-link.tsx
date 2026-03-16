"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { useAuth } from "@/lib/auth-context";

interface Props {
  ownerUsername: string;
}

/**
 * Renders a "Board settings" link only when the authenticated viewer
 * is the board owner.
 */
export function BoardSettingsLink({ ownerUsername }: Props) {
  const t = useTranslations("boardSettings");
  const { user } = useAuth();
  if (!user || user.username !== ownerUsername) return null;

  return (
    <Link
      href="/settings/board"
      className="flex items-center gap-1.5 rounded-md border border-[#2a2e38] bg-[#0f1117] px-3 py-1.5 text-xs text-[#aeb4bf] hover:border-[#3a3f4e] hover:text-white transition-colors"
    >
      <span aria-hidden>⚙</span>
      {t("boardSettingsLink")}
    </Link>
  );
}
