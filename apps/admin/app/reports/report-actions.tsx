"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

const RESOLVE_MUTATION = `
  mutation ResolveReports($postId: ID!, $note: String) {
    resolveReports(postId: $postId, note: $note)
  }
`;
const DISMISS_MUTATION = `
  mutation DismissReports($postId: ID!, $note: String) {
    dismissReports(postId: $postId, note: $note)
  }
`;
const DELETE_MUTATION = `
  mutation DeletePost($postId: ID!, $note: String) {
    deletePost(postId: $postId, note: $note)
  }
`;

async function callMutation(query: string, variables: Record<string, unknown>) {
  const res = await fetch("/api/admin/graphql", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ query, variables }),
    credentials: "include",
  });
  const json = await res.json() as { errors?: { message: string }[] };
  if (json.errors?.length) throw new Error(json.errors[0].message);
}

export function ReportActions({ postId }: { postId: string }) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);

  async function act(mutation: string, label: string) {
    if (!confirm(`${label} — are you sure?`)) return;
    setBusy(true);
    try {
      await callMutation(mutation, { postId });
      router.refresh();
    } catch (err) {
      alert(err instanceof Error ? err.message : "Action failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex items-center gap-2">
      <button
        disabled={busy}
        onClick={() => act(DISMISS_MUTATION, "Dismiss all reports")}
        className="rounded border border-zinc-200 px-2.5 py-1 text-xs text-zinc-500 hover:bg-zinc-50 disabled:opacity-40"
      >
        Dismiss
      </button>
      <button
        disabled={busy}
        onClick={() => act(RESOLVE_MUTATION, "Resolve reports (keep post)")}
        className="rounded border border-zinc-200 px-2.5 py-1 text-xs text-zinc-600 hover:bg-zinc-50 disabled:opacity-40"
      >
        Resolve
      </button>
      <button
        disabled={busy}
        onClick={() => act(DELETE_MUTATION, "Delete post")}
        className="rounded border border-red-200 bg-red-50 px-2.5 py-1 text-xs font-medium text-red-600 hover:bg-red-100 disabled:opacity-40"
      >
        Delete post
      </button>
    </div>
  );
}
