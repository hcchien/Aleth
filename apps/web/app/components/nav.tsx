"use client";

import Link from "next/link";
import { useAuth } from "@/lib/auth-context";

export function Nav() {
  const { user, logout, loading } = useAuth();

  return (
    <nav className="border-b border-gray-200 bg-white sticky top-0 z-10">
      <div className="max-w-2xl mx-auto px-4 h-12 flex items-center justify-between">
        <Link href="/" className="font-semibold text-lg tracking-tight">
          Aleth
        </Link>

        <div className="flex items-center gap-4 text-sm">
          {loading ? null : user ? (
            <>
              <Link
                href="/compose"
                className="text-gray-600 hover:text-gray-900"
              >
                Compose
              </Link>
              <Link
                href={`/@${user.username}`}
                className="text-gray-600 hover:text-gray-900"
              >
                {user.displayName ?? user.username}
              </Link>
              <button
                onClick={logout}
                className="text-gray-400 hover:text-gray-700"
              >
                Sign out
              </button>
            </>
          ) : (
            <>
              <Link href="/login" className="text-gray-600 hover:text-gray-900">
                Sign in
              </Link>
              <Link
                href="/register"
                className="bg-gray-900 text-white px-3 py-1.5 rounded-md hover:bg-gray-700"
              >
                Sign up
              </Link>
            </>
          )}
        </div>
      </div>
    </nav>
  );
}
