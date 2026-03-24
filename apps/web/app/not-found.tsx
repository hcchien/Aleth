import Link from "next/link";

export default function NotFound() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-[var(--app-bg)] px-4 text-center">
      {/* Large editorial number */}
      <p
        className="select-none text-[10rem] font-black leading-none tracking-tighter text-[var(--app-surface-4)]"
        aria-hidden="true"
        style={{ fontFamily: "var(--font-manrope), 'Manrope', sans-serif" }}
      >
        404
      </p>

      <div className="-mt-4 max-w-sm">
        <h1
          className="mb-3 text-2xl font-bold text-[var(--app-text-heading)]"
          style={{ fontFamily: "var(--font-newsreader), 'Newsreader', Georgia, serif" }}
        >
          Page not found
        </h1>
        <p className="mb-8 text-sm leading-relaxed text-[var(--app-text-muted)]">
          The page you&apos;re looking for doesn&apos;t exist or may have been removed.
        </p>

        <div className="flex justify-center gap-3">
          <Link
            href="/"
            className="rounded-xl bg-[var(--app-accent)] px-5 py-2.5 text-sm font-bold text-white hover:opacity-90 transition-opacity"
          >
            Go home
          </Link>
          <Link
            href="javascript:history.back()"
            className="rounded-xl border border-[var(--app-border-inner)] px-5 py-2.5 text-sm font-medium text-[var(--app-text-secondary)] hover:border-[var(--app-border-hover)] transition-colors"
          >
            Go back
          </Link>
        </div>
      </div>

      {/* Subtle brand anchor */}
      <p className="mt-16 text-[10px] font-bold uppercase tracking-widest text-[var(--app-text-dim)]">
        Aleth
      </p>
    </div>
  );
}
