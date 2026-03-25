"use client";

import { useState, FormEvent } from "react";
import Link from "next/link";
import { gqlClient } from "@/lib/gql-client";

const REQUEST_PASSWORD_RESET_MUTATION = `
  mutation RequestPasswordReset($email: String!) {
    requestPasswordReset(email: $email)
  }
`;

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [submitted, setSubmitted] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      await gqlClient<{ requestPasswordReset: boolean }>(
        REQUEST_PASSWORD_RESET_MUTATION,
        { email }
      );
      setSubmitted(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong. Please try again.");
    } finally {
      setLoading(false);
    }
  }

  if (submitted) {
    return (
      <div className="max-w-sm mx-auto mt-12">
        <h1 className="text-2xl font-semibold mb-4">Check your email</h1>
        <p className="text-sm text-gray-600 mb-6">
          If an account exists for <strong>{email}</strong>, you will receive a
          password reset link shortly.
        </p>
        <p className="text-sm text-gray-500">
          <Link href="/login" className="text-gray-900 hover:underline">
            Back to sign in
          </Link>
        </p>
      </div>
    );
  }

  return (
    <div className="max-w-sm mx-auto mt-12">
      <h1 className="text-2xl font-semibold mb-2">Forgot your password?</h1>
      <p className="text-sm text-gray-500 mb-6">
        Enter your email address and we will send you a reset link.
      </p>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Email
          </label>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            placeholder="you@example.com"
            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"
          />
        </div>
        {error && <p className="text-red-600 text-sm">{error}</p>}
        <button
          type="submit"
          disabled={loading}
          className="bg-gray-900 text-white rounded-md px-4 py-2 text-sm font-medium hover:bg-gray-700 disabled:opacity-50"
        >
          {loading ? "Sending…" : "Send reset link"}
        </button>
      </form>
      <p className="mt-4 text-sm text-gray-500">
        Remember your password?{" "}
        <Link href="/login" className="text-gray-900 hover:underline">
          Sign in
        </Link>
      </p>
    </div>
  );
}
