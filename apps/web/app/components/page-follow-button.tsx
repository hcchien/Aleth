"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { gqlClient } from "@/lib/gql-client";
import { useAuth } from "@/lib/auth-context";

const PAGE_FOLLOW_STATUS_QUERY = `
  query PageFollowStatus($slug: String!) {
    page(slug: $slug) {
      id
      followerCount
      isFollowing
    }
  }
`;

const FOLLOW_PAGE = `mutation FollowPage($pageId: ID!) { followPage(pageId: $pageId) }`;
const UNFOLLOW_PAGE = `mutation UnfollowPage($pageId: ID!) { unfollowPage(pageId: $pageId) }`;

interface Props {
  pageId: string;
  slug: string;
  initialFollowerCount: number;
  initialIsFollowing?: boolean;
}

export function PageFollowButton({ pageId, slug, initialFollowerCount, initialIsFollowing }: Props) {
  const t = useTranslations("fanPage");
  const { user } = useAuth();
  const [following, setFollowing] = useState(initialIsFollowing ?? false);
  const [count, setCount] = useState(initialFollowerCount);
  const [pending, setPending] = useState(false);
  const [ready, setReady] = useState(!user);

  useEffect(() => {
    if (!user) {
      setReady(true);
      return;
    }
    gqlClient<{ page: { id: string; followerCount: number; isFollowing: boolean } | null }>(
      PAGE_FOLLOW_STATUS_QUERY,
      { slug }
    )
      .then((data) => {
        if (data.page) {
          setFollowing(data.page.isFollowing);
          setCount(data.page.followerCount);
        }
      })
      .catch(() => {})
      .finally(() => setReady(true));
  }, [user, slug]);

  if (!user) return null;
  if (!ready) {
    return <div className="h-8 w-28 animate-pulse rounded-full bg-[var(--app-border)]" />;
  }

  async function toggle() {
    if (pending) return;
    setPending(true);
    const wasFollowing = following;
    setFollowing(!wasFollowing);
    setCount((c) => c + (wasFollowing ? -1 : 1));
    try {
      await gqlClient(wasFollowing ? UNFOLLOW_PAGE : FOLLOW_PAGE, { pageId });
    } catch {
      setFollowing(wasFollowing);
      setCount((c) => c + (wasFollowing ? 1 : -1));
    } finally {
      setPending(false);
    }
  }

  return (
    <button
      onClick={toggle}
      disabled={pending}
      className={`shrink-0 rounded-full border px-4 py-1.5 text-sm transition-colors disabled:opacity-50 ${
        following
          ? "border-[var(--app-border-inner)] text-[var(--app-text-secondary)] hover:border-red-900/60 hover:text-red-400"
          : "border-[var(--app-accent)]/50 bg-[var(--app-accent-bg)] text-[var(--app-accent)] hover:opacity-90"
      }`}
    >
      {following
        ? `${t("following")} · ${count}`
        : `${t("follow")} · ${count}`}
    </button>
  );
}
