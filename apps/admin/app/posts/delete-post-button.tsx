"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

export function DeletePostButton({ postId }: { postId: string }) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);

  async function handleDelete() {
    if (!confirm("Permanently delete this post?")) return;
    setBusy(true);
    try {
      const res = await fetch("/api/admin/graphql", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          query: `mutation DeletePost($postId: ID!) { deletePost(postId: $postId) }`,
          variables: { postId },
        }),
        credentials: "include",
      });
      const json = await res.json() as { errors?: { message: string }[] };
      if (json.errors?.length) throw new Error(json.errors[0].message);
      router.refresh();
    } catch (err) {
      alert(err instanceof Error ? err.message : "Failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <button
      disabled={busy}
      onClick={handleDelete}
      className="rounded border border-red-200 px-2.5 py-1 text-xs text-red-500 hover:bg-red-50 disabled:opacity-40"
    >
      {busy ? "…" : "Delete"}
    </button>
  );
}
