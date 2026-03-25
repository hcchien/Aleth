"use client";

import { useState } from "react";
import Link from "next/link";
import { useAuth } from "@/lib/auth-context";
import { gqlClient } from "@/lib/gql-client";

const RESEND_MUTATION = `
  mutation ResendVerificationEmail {
    resendVerificationEmail
  }
`;

export function EmailVerifyBanner() {
  const { user } = useAuth();
  const [sent, setSent] = useState(false);
  const [loading, setLoading] = useState(false);
  const [dismissed, setDismissed] = useState(false);

  // Only show for logged-in, unverified users who haven't dismissed
  if (!user || user.emailVerified || dismissed) return null;

  async function handleResend() {
    setLoading(true);
    try {
      await gqlClient(RESEND_MUTATION, {});
      setSent(true);
    } catch {
      // silent — user can retry
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="fixed top-16 left-0 right-0 z-40 bg-amber-50 dark:bg-amber-950/40 border-b border-amber-200 dark:border-amber-800 shadow-sm">
      <div className="max-w-5xl mx-auto px-4 py-2.5 flex items-center justify-between gap-4 text-sm">
        <p className="text-amber-800 dark:text-amber-200">
          Please verify your email address to unlock all features.{" "}
          {user.email && (
            <span className="text-amber-700 dark:text-amber-300">
              We sent a link to <strong>{user.email}</strong>.
            </span>
          )}
        </p>

        <div className="flex items-center gap-3 shrink-0">
          {sent ? (
            <span className="text-amber-700 dark:text-amber-300 font-medium">
              Link sent!
            </span>
          ) : (
            <button
              onClick={handleResend}
              disabled={loading}
              className="text-amber-800 dark:text-amber-200 font-medium underline underline-offset-2 hover:no-underline disabled:opacity-50"
            >
              {loading ? "Sending…" : "Resend link"}
            </button>
          )}

          <Link
            href="/verify-email"
            className="text-amber-800 dark:text-amber-200 font-medium underline underline-offset-2 hover:no-underline"
          >
            Help
          </Link>

          <button
            onClick={() => setDismissed(true)}
            aria-label="Dismiss"
            className="text-amber-600 dark:text-amber-400 hover:text-amber-900 dark:hover:text-amber-100"
          >
            ✕
          </button>
        </div>
      </div>
    </div>
  );
}
