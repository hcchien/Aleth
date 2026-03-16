"use client";

import { useState, FormEvent, useEffect } from "react";
import { useRouter } from "next/navigation";
import { gqlClient } from "@/lib/gql-client";
import { useAuth } from "@/lib/auth-context";

const CREATE_POST_MUTATION = `
  mutation CreatePost($input: CreatePostInput!) {
    createPost(input: $input) {
      id
    }
  }
`;

export default function ComposePage() {
  const router = useRouter();
  const { user, loading } = useAuth();
  const [content, setContent] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!loading && !user) {
      router.replace("/login");
    }
  }, [loading, user, router]);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!content.trim()) return;
    setError(null);
    setSubmitting(true);
    try {
      await gqlClient(CREATE_POST_MUTATION, { input: { content } });
      router.push("/");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to post");
    } finally {
      setSubmitting(false);
    }
  }

  if (loading || !user) return null;

  return (
    <div className="max-w-lg mx-auto mt-8">
      <h1 className="text-xl font-semibold mb-4">New post</h1>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <textarea
          value={content}
          onChange={(e) => setContent(e.target.value)}
          placeholder="What's on your mind?"
          rows={5}
          maxLength={1000}
          required
          className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-gray-900"
        />
        <div className="flex items-center justify-between">
          <span className="text-xs text-gray-400">{content.length}/1000</span>
          <div className="flex gap-3">
            <button
              type="button"
              onClick={() => router.back()}
              className="text-sm text-gray-500 hover:text-gray-700"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={submitting || !content.trim()}
              className="bg-gray-900 text-white rounded-md px-4 py-2 text-sm font-medium hover:bg-gray-700 disabled:opacity-50"
            >
              {submitting ? "Posting…" : "Post"}
            </button>
          </div>
        </div>
        {error && <p className="text-red-600 text-sm">{error}</p>}
      </form>
    </div>
  );
}
