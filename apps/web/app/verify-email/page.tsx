"use client";

import { useEffect, useState, Suspense } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import Link from "next/link";
import { gqlClient } from "@/lib/gql-client";

const VERIFY_EMAIL_MUTATION = `
  mutation VerifyEmail($token: String!) {
    verifyEmail(token: $token)
  }
`;

const RESEND_MUTATION = `
  mutation ResendVerificationEmail {
    resendVerificationEmail
  }
`;

function VerifyEmailContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const token = searchParams.get("token");

  const [status, setStatus] = useState<"verifying" | "success" | "error" | "no-token">(
    token ? "verifying" : "no-token"
  );
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [resendLoading, setResendLoading] = useState(false);
  const [resendSent, setResendSent] = useState(false);

  useEffect(() => {
    if (!token) return;

    gqlClient<{ verifyEmail: boolean }>(VERIFY_EMAIL_MUTATION, { token })
      .then(() => {
        setStatus("success");
        setTimeout(() => router.push("/"), 3000);
      })
      .catch((err) => {
        setStatus("error");
        setErrorMsg(err instanceof Error ? err.message : "Verification failed");
      });
  }, [token, router]);

  async function handleResend() {
    setResendLoading(true);
    try {
      await gqlClient(RESEND_MUTATION, {});
      setResendSent(true);
    } catch {
      // ignore — user may not be logged in
    } finally {
      setResendLoading(false);
    }
  }

  return (
    <div className="max-w-sm mx-auto mt-20 text-center px-4">
      {status === "verifying" && (
        <>
          <div className="w-8 h-8 border-2 border-[var(--app-accent)] border-t-transparent rounded-full animate-spin mx-auto mb-4" />
          <p className="text-[var(--app-muted)] text-sm">Verifying your email…</p>
        </>
      )}

      {status === "success" && (
        <>
          <div className="text-4xl mb-4">✓</div>
          <h1 className="text-xl font-semibold mb-2 text-[var(--app-fg)]">Email verified!</h1>
          <p className="text-[var(--app-muted)] text-sm mb-6">
            Your email address has been confirmed. Redirecting you home…
          </p>
          <Link
            href="/"
            className="inline-block bg-[var(--app-fg)] text-[var(--app-bg)] text-sm font-medium px-5 py-2 rounded-md hover:opacity-80"
          >
            Go home
          </Link>
        </>
      )}

      {status === "error" && (
        <>
          <h1 className="text-xl font-semibold mb-2 text-[var(--app-fg)]">Link expired or invalid</h1>
          <p className="text-[var(--app-muted)] text-sm mb-6">
            {errorMsg ?? "This verification link is no longer valid."}
          </p>
          {resendSent ? (
            <p className="text-sm text-[var(--app-fg)]">
              A new verification link has been sent to your email.
            </p>
          ) : (
            <button
              onClick={handleResend}
              disabled={resendLoading}
              className="bg-[var(--app-fg)] text-[var(--app-bg)] text-sm font-medium px-5 py-2 rounded-md hover:opacity-80 disabled:opacity-50"
            >
              {resendLoading ? "Sending…" : "Resend verification email"}
            </button>
          )}
          <p className="mt-4 text-sm text-[var(--app-muted)]">
            <Link href="/login" className="hover:underline">Back to sign in</Link>
          </p>
        </>
      )}

      {status === "no-token" && (
        <>
          <h1 className="text-xl font-semibold mb-2 text-[var(--app-fg)]">Check your email</h1>
          <p className="text-[var(--app-muted)] text-sm mb-6">
            We sent a verification link to your email address. Click the link to confirm your account.
          </p>
          {resendSent ? (
            <p className="text-sm text-[var(--app-fg)]">A new link has been sent.</p>
          ) : (
            <button
              onClick={handleResend}
              disabled={resendLoading}
              className="text-sm text-[var(--app-muted)] hover:text-[var(--app-fg)] underline disabled:opacity-50"
            >
              {resendLoading ? "Sending…" : "Resend verification email"}
            </button>
          )}
          <p className="mt-4 text-sm text-[var(--app-muted)]">
            <Link href="/login" className="hover:underline">Back to sign in</Link>
          </p>
        </>
      )}
    </div>
  );
}

export default function VerifyEmailPage() {
  return (
    <Suspense fallback={
      <div className="max-w-sm mx-auto mt-20 text-center px-4">
        <div className="w-8 h-8 border-2 border-[var(--app-accent)] border-t-transparent rounded-full animate-spin mx-auto" />
      </div>
    }>
      <VerifyEmailContent />
    </Suspense>
  );
}
