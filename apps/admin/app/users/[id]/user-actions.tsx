"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

async function mutate(query: string, variables: Record<string, unknown>) {
  const res = await fetch("/api/admin/graphql", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ query, variables }),
    credentials: "include",
  });
  const json = await res.json() as { errors?: { message: string }[] };
  if (json.errors?.length) throw new Error(json.errors[0].message);
}

export function UserActions({
  userId,
  trustLevel,
  isSuspended,
}: {
  userId: string;
  trustLevel: number;
  isSuspended: boolean;
}) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);

  async function setTrust() {
    const input = prompt(`Set trust level (0–5). Current: ${trustLevel}`);
    if (input === null) return;
    const lvl = parseInt(input);
    if (isNaN(lvl) || lvl < 0 || lvl > 5) { alert("Invalid trust level"); return; }
    setBusy(true);
    try {
      await mutate(`mutation SetTrust($userId: ID!, $trustLevel: Int!) {
        setTrustLevel(userId: $userId, trustLevel: $trustLevel)
      }`, { userId, trustLevel: lvl });
      router.refresh();
    } catch (err) {
      alert(err instanceof Error ? err.message : "Failed");
    } finally {
      setBusy(false);
    }
  }

  async function toggleSuspend() {
    const action = isSuspended ? "Unsuspend" : "Suspend";
    if (!confirm(`${action} this user?`)) return;
    const mutation = isSuspended
      ? `mutation Unsuspend($userId: ID!) { unsuspendUser(userId: $userId) }`
      : `mutation Suspend($userId: ID!) { suspendUser(userId: $userId) }`;
    setBusy(true);
    try {
      await mutate(mutation, { userId });
      router.refresh();
    } catch (err) {
      alert(err instanceof Error ? err.message : "Failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex items-center gap-2">
      <button
        disabled={busy}
        onClick={setTrust}
        className="rounded border border-zinc-200 px-3 py-1.5 text-xs text-zinc-600 hover:bg-zinc-50 disabled:opacity-40"
      >
        Set trust level
      </button>
      <button
        disabled={busy}
        onClick={toggleSuspend}
        className={`rounded border px-3 py-1.5 text-xs font-medium disabled:opacity-40 ${
          isSuspended
            ? "border-green-200 bg-green-50 text-green-700 hover:bg-green-100"
            : "border-red-200 bg-red-50 text-red-600 hover:bg-red-100"
        }`}
      >
        {isSuspended ? "Unsuspend" : "Suspend"}
      </button>
    </div>
  );
}
