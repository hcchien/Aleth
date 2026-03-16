"use client";

import { useState } from "react";
import { gqlClient } from "@/lib/gql-client";
import { useAuth } from "@/lib/auth-context";

const REACT_POST = `mutation ReactPost($postId: ID!, $emotion: String!) { reactPost(postId: $postId, emotion: $emotion) }`;
const UNREACT_POST = `mutation UnreactPost($postId: ID!) { unreactPost(postId: $postId) }`;

const EMOTIONS = ["like", "love", "haha", "wow", "sad", "angry"] as const;
const EMOJI: Record<(typeof EMOTIONS)[number], string> = {
  like: "👍",
  love: "❤️",
  haha: "😂",
  wow: "😮",
  sad: "😢",
  angry: "😡",
};
const LABEL: Record<(typeof EMOTIONS)[number], string> = {
  like: "Like",
  love: "Love",
  haha: "Haha",
  wow: "Wow",
  sad: "Sad",
  angry: "Angry",
};

interface LikeButtonProps {
  postId: string;
  initialViewerEmotion?: string | null;
  initialReactionCounts: { emotion: string; count: number }[];
}

export function LikeButton({
  postId,
  initialViewerEmotion,
  initialReactionCounts,
}: LikeButtonProps) {
  const { user } = useAuth();
  const [viewerEmotion, setViewerEmotion] = useState<string | null>(initialViewerEmotion ?? null);
  const [counts, setCounts] = useState<Record<string, number>>(() => {
    const out: Record<string, number> = {};
    for (const e of EMOTIONS) out[e] = 0;
    for (const r of initialReactionCounts) out[r.emotion] = r.count;
    return out;
  });
  const [pending, setPending] = useState(false);
  const [open, setOpen] = useState(false);

  const totalCount = EMOTIONS.reduce((sum, emotion) => sum + (counts[emotion] || 0), 0);
  const topEmotions = [...EMOTIONS]
    .map((emotion) => ({ emotion, count: counts[emotion] || 0 }))
    .filter((item) => item.count > 0)
    .sort((a, b) => b.count - a.count)
    .slice(0, 3);

  function adjust(oldEmotion: string | null, newEmotion: string | null) {
    setCounts((prev) => {
      const next = { ...prev };
      if (oldEmotion) next[oldEmotion] = Math.max(0, (next[oldEmotion] || 0) - 1);
      if (newEmotion) next[newEmotion] = (next[newEmotion] || 0) + 1;
      return next;
    });
  }

  async function react(emotion: (typeof EMOTIONS)[number] | "") {
    if (!user || pending) return;
    setPending(true);
    const oldEmotion = viewerEmotion;
    const nextEmotion = emotion === "" ? null : oldEmotion === emotion ? null : emotion;
    setViewerEmotion(nextEmotion);
    adjust(oldEmotion, nextEmotion);
    try {
      if (nextEmotion === null) {
        await gqlClient(UNREACT_POST, { postId });
      } else {
        await gqlClient(REACT_POST, { postId, emotion: nextEmotion });
      }
    } catch {
      setViewerEmotion(oldEmotion);
      adjust(nextEmotion, oldEmotion);
    } finally {
      setPending(false);
      setOpen(false);
    }
  }

  return (
    <div className="relative flex items-center gap-2">
      <button
        type="button"
        disabled={!user || pending}
        onClick={() => setOpen((v) => !v)}
        onMouseEnter={() => user && setOpen(true)}
        className={`rounded-full border px-2 py-1 text-xs disabled:opacity-60 ${
          viewerEmotion ? "border-blue-300 bg-blue-50 text-blue-700" : "border-gray-300 bg-white text-gray-700"
        }`}
      >
        {viewerEmotion ? `${EMOJI[viewerEmotion as (typeof EMOTIONS)[number]]} ${LABEL[viewerEmotion as (typeof EMOTIONS)[number]]}` : "React"}
      </button>
      {open && user && (
        <div
          className="absolute -top-12 left-0 z-30 flex items-center gap-1 rounded-full border border-gray-200 bg-white px-2 py-1 shadow-lg"
          onMouseLeave={() => setOpen(false)}
        >
          {EMOTIONS.map((emotion) => (
            <button
              key={emotion}
              type="button"
              onClick={() => react(emotion)}
              className="rounded-full p-1 text-lg transition-transform hover:scale-125"
              title={LABEL[emotion]}
            >
              {EMOJI[emotion]}
            </button>
          ))}
          <button
            type="button"
            onClick={() => react("")}
            className="ml-1 rounded-full border border-gray-200 px-2 py-0.5 text-[10px] text-gray-500 hover:bg-gray-100"
            title="Remove reaction"
          >
            Clear
          </button>
        </div>
      )}
      <div className="flex flex-wrap gap-1">
        {totalCount > 0 ? (
          <span className="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-1 text-xs text-gray-700">
            <span className="flex -space-x-1">
              {topEmotions.map((item) => (
                <span
                  key={item.emotion}
                  className="inline-flex h-4 w-4 items-center justify-center rounded-full border border-white bg-white text-[10px]"
                  title={`${item.emotion} ${item.count}`}
                >
                  {EMOJI[item.emotion]}
                </span>
              ))}
            </span>
            <span>{totalCount}</span>
          </span>
        ) : (
          <span className="rounded-full bg-gray-100 px-2 py-1 text-xs text-gray-500">0</span>
        )}
      </div>
    </div>
  );
}
