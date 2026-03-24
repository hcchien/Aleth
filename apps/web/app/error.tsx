"use client";

import { useEffect } from "react";

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("[GlobalError]", error);
  }, [error]);

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-[var(--app-bg)] px-4 text-center">
      {/* Large editorial number */}
      <p
        className="select-none text-[10rem] font-black leading-none tracking-tighter text-[var(--app-surface-4)]"
        aria-hidden="true"
        style={{ fontFamily: "var(--font-manrope), 'Manrope', sans-serif" }}
      >
        500
      </p>

      <div className="-mt-4 max-w-sm">
        <h1
          className="mb-3 text-2xl font-bold text-[var(--app-text-heading)]"
          style={{ fontFamily: "var(--font-newsreader), 'Newsreader', Georgia, serif" }}
        >
          Something went wrong
        </h1>
        <p className="mb-8 text-sm leading-relaxed text-[var(--app-text-muted)]">
          An unexpected error occurred on our end. You can try again or return to the home page.
        </p>

        <div className="flex justify-center gap-3">
          <button
            type="button"
            onClick={reset}
            className="rounded-xl bg-[var(--app-accent)] px-5 py-2.5 text-sm font-bold text-white hover:opacity-90 transition-opacity"
          >
            Try again
          </button>
          <a
            href="/"
            className="rounded-xl border border-[var(--app-border-inner)] px-5 py-2.5 text-sm font-medium text-[var(--app-text-secondary)] hover:border-[var(--app-border-hover)] transition-colors"
          >
            Go home
          </a>
        </div>

        {error.digest && (
          <p className="mt-6 text-[10px] text-[var(--app-text-dim)] font-mono">
            Error ID: {error.digest}
          </p>
        )}
      </div>

      {/* Subtle brand anchor */}
      <p className="mt-16 text-[10px] font-bold uppercase tracking-widest text-[var(--app-text-dim)]">
        Aleth
      </p>
    </div>
  );
}
