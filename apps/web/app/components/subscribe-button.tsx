"use client";

import { useState } from "react";
import { gqlClient } from "@/lib/gql-client";
import { useAuth } from "@/lib/auth-context";

const SUBSCRIBE = `mutation SubscribeBoard($ownerID: ID!) { subscribeBoard(ownerID: $ownerID) }`;
const UNSUBSCRIBE = `mutation UnsubscribeBoard($ownerID: ID!) { unsubscribeBoard(ownerID: $ownerID) }`;

interface SubscribeButtonProps {
  ownerID: string;
  initialSubscribed: boolean;
  initialCount: number;
}

export function SubscribeButton({
  ownerID,
  initialSubscribed,
  initialCount,
}: SubscribeButtonProps) {
  const { user } = useAuth();
  const [subscribed, setSubscribed] = useState(initialSubscribed);
  const [count, setCount] = useState(initialCount);
  const [pending, setPending] = useState(false);

  if (!user) return null;

  async function toggle() {
    if (pending) return;
    setPending(true);
    const wasSubscribed = subscribed;
    setSubscribed(!wasSubscribed);
    setCount((c) => c + (wasSubscribed ? -1 : 1));
    try {
      await gqlClient(wasSubscribed ? UNSUBSCRIBE : SUBSCRIBE, { ownerID });
    } catch {
      setSubscribed(wasSubscribed);
      setCount((c) => c + (wasSubscribed ? 1 : -1));
    } finally {
      setPending(false);
    }
  }

  return (
    <button
      onClick={toggle}
      disabled={pending}
      className={`shrink-0 rounded-full border px-4 py-1.5 text-sm transition-colors disabled:opacity-50 ${
        subscribed
          ? "border-[#3a3f48] text-[#9ea4b0] hover:border-red-900/60 hover:text-red-400"
          : "border-[#e89246]/50 bg-[#2a1f18] text-[#e89246] hover:bg-[#3a2a18]"
      }`}
    >
      {subscribed ? `已追蹤 · ${count}` : `追蹤 · ${count}`}
    </button>
  );
}
