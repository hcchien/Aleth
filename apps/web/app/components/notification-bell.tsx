"use client";

import { useEffect, useRef, useState } from "react";
import { getAccessToken } from "@/lib/auth";

interface Notification {
  id: string;
  type: string; // 'reply' | 'reshare' | 'comment' | 'reaction' | 'page_post'
  actor_id: string;
  entity_type: string;
  entity_id: string;
  read: boolean;
  created_at: string;
}

async function notifFetch(path: string, options?: RequestInit) {
  const token = getAccessToken();
  return fetch(`/api/notifications${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options?.headers,
    },
  });
}

const TYPE_LABEL: Record<string, string> = {
  reply: "回覆了你的貼文",
  reshare: "轉貼了你的貼文",
  comment: "在文章留言",
  reaction: "對你的貼文按讚",
  page_post: "在頁面發佈了新文章",
};

function formatRelativeTime(iso: string) {
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60_000);
  if (mins < 1) return "剛剛";
  if (mins < 60) return `${mins} 分鐘前`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs} 小時前`;
  return `${Math.floor(hrs / 24)} 天前`;
}

export function NotificationBell() {
  const [unread, setUnread] = useState(0);
  const [open, setOpen] = useState(false);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [loading, setLoading] = useState(false);
  const panelRef = useRef<HTMLDivElement>(null);

  // Fetch unread count on mount.
  useEffect(() => {
    notifFetch("/count")
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (data) setUnread(data.unread ?? 0);
      })
      .catch(() => {});
  }, []);

  // Close on outside click.
  useEffect(() => {
    if (!open) return;
    function handler(e: MouseEvent) {
      if (panelRef.current && !panelRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [open]);

  async function openPanel() {
    if (open) {
      setOpen(false);
      return;
    }
    setOpen(true);
    setLoading(true);
    try {
      const [listRes] = await Promise.all([
        notifFetch("?limit=30"),
        // Mark all as read immediately when opening.
        unread > 0
          ? notifFetch("/mark-read", { method: "POST", body: JSON.stringify({}) })
          : Promise.resolve(),
      ]);
      if (listRes.ok) {
        const data: Notification[] = await listRes.json();
        setNotifications(data ?? []);
      }
      if (unread > 0) setUnread(0);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="relative" ref={panelRef}>
      <button
        type="button"
        onClick={openPanel}
        className="relative flex items-center justify-center rounded-sm p-1.5 text-lg text-[var(--app-text-nav)] hover:bg-[var(--app-hover)] hover:text-[var(--app-text-heading)]"
        aria-label="通知"
      >
        🔔
        {unread > 0 && (
          <span className="absolute -right-1 -top-1 flex h-4 min-w-4 items-center justify-center rounded-full bg-red-500 px-1 text-[10px] font-bold leading-none text-white">
            {unread > 99 ? "99+" : unread}
          </span>
        )}
      </button>

      {open && (
        <div className="absolute right-0 top-full z-50 mt-2 w-80 rounded-xl border border-[var(--app-border)] bg-[var(--app-header)] shadow-2xl">
          <div className="flex items-center justify-between border-b border-[var(--app-border)] px-4 py-3">
            <span className="text-sm font-semibold text-[var(--app-text)]">通知</span>
            {notifications.some((n) => !n.read) && (
              <button
                type="button"
                className="text-xs text-[var(--app-text-muted)] hover:text-[var(--app-text-nav)]"
                onClick={async () => {
                  await notifFetch("/mark-read", {
                    method: "POST",
                    body: JSON.stringify({}),
                  });
                  setNotifications((prev) => prev.map((n) => ({ ...n, read: true })));
                }}
              >
                全部標為已讀
              </button>
            )}
          </div>

          <div className="max-h-96 overflow-y-auto">
            {loading ? (
              <div className="px-4 py-6 text-center text-sm text-[var(--app-text-muted)]">
                載入中…
              </div>
            ) : notifications.length === 0 ? (
              <div className="px-4 py-6 text-center text-sm text-[var(--app-text-muted)]">
                目前沒有通知
              </div>
            ) : (
              notifications.map((n) => (
                <NotificationItem key={n.id} notification={n} />
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function NotificationItem({ notification: n }: { notification: Notification }) {
  const label = TYPE_LABEL[n.type] ?? n.type;
  const entityPath =
    n.entity_type === "post"
      ? `/posts/${n.entity_id}`
      : `/posts/${n.entity_id}`;

  return (
    <a
      href={entityPath}
      className={`flex items-start gap-3 px-4 py-3 transition-colors hover:bg-[var(--app-hover)] ${
        !n.read ? "bg-[var(--app-hover-2)]" : ""
      }`}
    >
      <span className="mt-0.5 text-base">
        {n.type === "reply"
          ? "↩"
          : n.type === "reshare"
          ? "↗"
          : n.type === "comment"
          ? "💬"
          : n.type === "page_post"
          ? "📢"
          : "❤️"}
      </span>
      <div className="flex-1 min-w-0">
        <p className="text-sm text-[var(--app-text-bright)]">{label}</p>
        <p className="mt-0.5 text-xs text-[var(--app-text-muted)]">
          {formatRelativeTime(n.created_at)}
        </p>
      </div>
      {!n.read && (
        <span className="mt-1.5 h-2 w-2 flex-shrink-0 rounded-full bg-blue-500" />
      )}
    </a>
  );
}
