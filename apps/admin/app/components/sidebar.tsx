"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const NAV = [
  { href: "/",        label: "Dashboard",   icon: "▣" },
  { href: "/reports", label: "Reports",     icon: "⚑" },
  { href: "/users",   label: "Users",       icon: "⊙" },
  { href: "/posts",   label: "Posts",       icon: "≡" },
  { href: "/audit",   label: "Audit Log",   icon: "◎" },
];

export function Sidebar() {
  const path = usePathname();
  return (
    <aside className="flex h-screen w-56 flex-col border-r border-zinc-200 bg-white">
      <div className="px-6 py-5 border-b border-zinc-200">
        <span className="text-sm font-semibold tracking-widest text-zinc-400 uppercase">Aleth Admin</span>
      </div>
      <nav className="flex-1 px-3 py-4 space-y-0.5">
        {NAV.map(({ href, label, icon }) => {
          const active = href === "/" ? path === "/" : path.startsWith(href);
          return (
            <Link
              key={href}
              href={href}
              className={`flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors ${
                active
                  ? "bg-zinc-100 font-medium text-zinc-900"
                  : "text-zinc-500 hover:bg-zinc-50 hover:text-zinc-900"
              }`}
            >
              <span className="text-base leading-none">{icon}</span>
              {label}
            </Link>
          );
        })}
      </nav>
      <form action="/api/admin/logout" method="post" className="px-3 pb-4">
        <button
          type="submit"
          className="w-full rounded-md px-3 py-2 text-left text-sm text-zinc-400 hover:bg-zinc-50 hover:text-zinc-700 transition-colors"
        >
          ↪ Sign out
        </button>
      </form>
    </aside>
  );
}
