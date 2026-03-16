"use client";

import { useEffect, useState } from "react";
import { gqlClient } from "@/lib/gql-client";
import { useAuth } from "@/lib/auth-context";

const FOLLOW_STATS_QUERY = `
  query FollowStats($userID: ID!) {
    followStats(userID: $userID) {
      followerCount
      isFollowing
    }
  }
`;

const FOLLOW = `mutation FollowUser($userID: ID!) { followUser(userID: $userID) }`;
const UNFOLLOW = `mutation UnfollowUser($userID: ID!) { unfollowUser(userID: $userID) }`;

interface Props {
  userID: string;
  ownerUsername: string;
  initialFollowerCount: number;
}

export function FollowButton({ userID, ownerUsername, initialFollowerCount }: Props) {
  const { user } = useAuth();
  const [following, setFollowing] = useState(false);
  const [count, setCount] = useState(initialFollowerCount);
  const [pending, setPending] = useState(false);
  const [ready, setReady] = useState(false);

  // Fetch actual isFollowing state from the server once auth resolves.
  useEffect(() => {
    if (!user) {
      setReady(true);
      return;
    }
    gqlClient<{ followStats: { followerCount: number; isFollowing: boolean } }>(
      FOLLOW_STATS_QUERY,
      { userID }
    )
      .then((data) => {
        setFollowing(data.followStats.isFollowing);
        setCount(data.followStats.followerCount);
      })
      .catch(() => {})
      .finally(() => setReady(true));
  }, [user, userID]);

  // Don't show button for guests or own profile.
  if (!user || user.username === ownerUsername) return null;
  // Show skeleton while loading to avoid layout shift.
  if (!ready) {
    return <div className="h-8 w-24 animate-pulse rounded-full bg-[var(--app-border)]" />;
  }

  async function toggle() {
    if (pending) return;
    setPending(true);
    const wasFollowing = following;
    setFollowing(!wasFollowing);
    setCount((c) => c + (wasFollowing ? -1 : 1));
    try {
      await gqlClient(wasFollowing ? UNFOLLOW : FOLLOW, { userID });
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
      {following ? `フォロー中 · ${count}` : `フォロー · ${count}`}
    </button>
  );
}
