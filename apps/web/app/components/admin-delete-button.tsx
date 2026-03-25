"use client";

import { useState } from "react";
import { useAuth } from "@/lib/auth-context";
import { gqlClient } from "@/lib/gql-client";

const ADMIN_DELETE_POST_MUTATION = `
  mutation AdminDeletePost($id: ID!) {
    adminDeletePost(id: $id)
  }
`;

export function AdminDeleteButton({
  postId,
  onDeleted,
}: {
  postId: string;
  onDeleted?: () => void;
}) {
  const { user } = useAuth();
  const [deleting, setDeleting] = useState(false);
  const [deleted, setDeleted] = useState(false);

  // Only visible to trust level 5+ (moderators / admins).
  if (!user || user.trustLevel < 5) return null;
  if (deleted) return <span className="text-xs text-[var(--app-text-muted)]">[removed]</span>;

  async function handleDelete() {
    if (!confirm("Admin: permanently remove this post?")) return;
    setDeleting(true);
    try {
      await gqlClient<{ adminDeletePost: boolean }>(ADMIN_DELETE_POST_MUTATION, { id: postId });
      setDeleted(true);
      onDeleted?.();
    } catch {
      alert("Failed to delete post.");
    } finally {
      setDeleting(false);
    }
  }

  return (
    <button
      type="button"
      disabled={deleting}
      onClick={() => void handleDelete()}
      className="text-xs text-red-400 hover:text-red-600 transition-colors disabled:opacity-50"
      title="Admin: remove post"
    >
      {deleting ? "…" : "[mod]"}
    </button>
  );
}
