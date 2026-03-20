"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { useAuth } from "@/lib/auth-context";
import { gqlClient } from "@/lib/gql-client";

const FOLLOW_MUTATION = `
  mutation FollowRemoteActor($handle: String!) {
    followRemoteActor(handle: $handle)
  }
`;

const UNFOLLOW_MUTATION = `
  mutation UnfollowRemoteActor($actorURL: String!) {
    unfollowRemoteActor(actorURL: $actorURL)
  }
`;

const MY_REMOTE_FOLLOWING_QUERY = `
  query MyRemoteFollowing {
    myRemoteFollowing {
      actorURL
      handle
      accepted
      createdAt
    }
  }
`;

interface RemoteActor {
  actorURL: string;
  handle: string;
  accepted: boolean;
  createdAt: string;
}

export function RemoteFollows() {
  const { user } = useAuth();
  const [following, setFollowing] = useState<RemoteActor[]>([]);
  const [handle, setHandle] = useState("");
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const refreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const fetchFollowing = useCallback(async () => {
    if (!user) return;
    setLoading(true);
    try {
      const data = await gqlClient<{ myRemoteFollowing: RemoteActor[] }>(
        MY_REMOTE_FOLLOWING_QUERY
      );
      setFollowing(data.myRemoteFollowing ?? []);
    } catch {
      // silently fail on initial load
    } finally {
      setLoading(false);
    }
  }, [user]);

  useEffect(() => {
    fetchFollowing();
    return () => {
      if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current);
    };
  }, [fetchFollowing]);

  async function follow(e: React.FormEvent) {
    e.preventDefault();
    if (!handle.trim()) return;
    setSubmitting(true);
    setError(null);
    setSuccess(null);
    try {
      await gqlClient(FOLLOW_MUTATION, { handle: handle.trim() });
      setSuccess(`Follow request sent to ${handle.trim()}. Waiting for acceptance.`);
      setHandle("");
      // Refresh list after a short delay so the DB record is visible
      refreshTimerRef.current = setTimeout(fetchFollowing, 800);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to follow account");
    } finally {
      setSubmitting(false);
    }
  }

  async function unfollow(actorURL: string) {
    setError(null);
    try {
      await gqlClient(UNFOLLOW_MUTATION, { actorURL });
      setFollowing((prev) => prev.filter((f) => f.actorURL !== actorURL));
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to unfollow");
    }
  }

  if (!user?.apEnabled) return null;

  return (
    <div className="mt-5 border-t border-[var(--app-border-2)] pt-5">
      <h3 className="mb-1 text-xs font-semibold text-[var(--app-text-bright)]">
        Follow Fediverse accounts
      </h3>
      <p className="mb-3 text-xs text-[var(--app-text-muted)]">
        Enter a handle like <span className="font-mono">@user@threads.net</span> or{" "}
        <span className="font-mono">@user@mastodon.social</span> to receive their posts here.
      </p>

      <form onSubmit={follow} className="flex gap-2">
        <input
          type="text"
          value={handle}
          onChange={(e) => setHandle(e.target.value)}
          placeholder="@user@threads.net"
          disabled={submitting}
          className="flex-1 rounded-lg border border-[var(--app-border-2)] bg-[var(--app-bg)] px-3 py-1.5 text-sm text-[var(--app-text)] placeholder:text-[var(--app-text-dim)] focus:border-[var(--app-accent)] focus:outline-none disabled:opacity-50"
        />
        <button
          type="submit"
          disabled={submitting || !handle.trim()}
          className="btn-outline text-sm disabled:opacity-40"
        >
          {submitting ? "Following…" : "Follow"}
        </button>
      </form>

      {error && <p className="mt-2 text-xs text-red-500">{error}</p>}
      {success && <p className="mt-2 text-xs text-green-600 dark:text-green-400">{success}</p>}

      {loading && (
        <p className="mt-3 text-xs text-[var(--app-text-dim)]">Loading…</p>
      )}

      {!loading && following.length > 0 && (
        <ul className="mt-3 space-y-2">
          {following.map((f) => (
            <li
              key={f.actorURL}
              className="flex items-center justify-between rounded-lg border border-[var(--app-border-2)] bg-[var(--app-bg)] px-3 py-2"
            >
              <div className="min-w-0">
                <span className="block truncate text-sm font-mono text-[var(--app-text)]">
                  {f.handle}
                </span>
                <span className="text-xs text-[var(--app-text-muted)]">
                  {f.accepted ? (
                    <span className="text-green-600 dark:text-green-400">Active</span>
                  ) : (
                    <span className="text-amber-600 dark:text-amber-400">Pending acceptance</span>
                  )}
                </span>
              </div>
              <button
                onClick={() => unfollow(f.actorURL)}
                className="ml-3 shrink-0 text-xs text-[var(--app-text-muted)] hover:text-red-500 transition-colors"
              >
                Unfollow
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
