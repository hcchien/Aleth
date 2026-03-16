"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useAuth } from "@/lib/auth-context";
import { gqlClient } from "@/lib/gql-client";
import { CreatePageModal } from "@/app/components/create-page-modal";

const MY_PAGES_QUERY = `
  query MyPages {
    myPages {
      id
      slug
      name
      category
      followerCount
    }
  }
`;

interface MyPage {
  id: string;
  slug: string;
  name: string;
  category: string;
  followerCount: number;
}

export default function SettingsPagesPage() {
  const t = useTranslations("fanPage");
  const tn = useTranslations("nav");
  const tCommon = useTranslations("common");
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();

  const [pages, setPages] = useState<MyPage[]>([]);
  const [fetching, setFetching] = useState(true);
  const [showCreate, setShowCreate] = useState(false);

  useEffect(() => {
    if (authLoading) return;
    if (!user) {
      router.replace("/login?redirect=/settings/pages");
      return;
    }

    gqlClient<{ myPages: MyPage[] }>(MY_PAGES_QUERY)
      .then((data) => setPages(data.myPages ?? []))
      .catch(() => {})
      .finally(() => setFetching(false));
  }, [user, authLoading]);

  if (authLoading || fetching) {
    return (
      <div className="mx-auto mt-10 max-w-2xl px-4 text-sm text-[#7a8090]">
        {tCommon("loading")}
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-2xl px-4 py-10 pb-20">
      {/* Breadcrumb */}
      <nav className="mb-8 flex items-center gap-2 text-sm text-[var(--app-text-muted)]">
        <Link href="/" className="hover:text-[var(--app-text-secondary)] transition-colors">
          {tn("feed")}
        </Link>
        <span>›</span>
        <Link href="/settings" className="hover:text-[var(--app-text-secondary)] transition-colors">
          Settings
        </Link>
        <span>›</span>
        <span className="text-[var(--app-text-secondary)]">{t("myPages")}</span>
      </nav>

      <div className="mb-8 flex items-center justify-between">
        <h1 className="font-serif text-3xl text-[var(--app-text-heading)]">{t("myPages")}</h1>
        <button
          onClick={() => setShowCreate(true)}
          className="rounded-md bg-[#f09a45] px-4 py-2 text-sm font-medium text-[#0b0d12] hover:bg-[#fbb468] transition-colors"
        >
          + {t("createPage")}
        </button>
      </div>

      {pages.length === 0 ? (
        <div className="rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)] px-6 py-12 text-center">
          <p className="mb-4 text-sm text-[var(--app-text-muted)]">{t("noPages")}</p>
          <button
            onClick={() => setShowCreate(true)}
            className="rounded-md bg-[#f09a45] px-5 py-2 text-sm font-medium text-[#0b0d12] hover:bg-[#fbb468] transition-colors"
          >
            {t("createPage")}
          </button>
        </div>
      ) : (
        <div className="space-y-3">
          {pages.map((p) => (
            <div
              key={p.id}
              className="flex items-center justify-between rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)] p-5 transition-colors hover:border-[var(--app-border-hover)]"
            >
              <div className="flex items-center gap-4">
                <div className="flex h-10 w-10 items-center justify-center rounded-full bg-[var(--app-accent-bg)] text-lg font-semibold text-[var(--app-accent)]">
                  {p.name[0]?.toUpperCase() ?? "P"}
                </div>
                <div>
                  <p className="font-medium text-[var(--app-text-bright)]">{p.name}</p>
                  <p className="text-xs text-[var(--app-text-muted)]">
                    /p/{p.slug} · {p.category} · {p.followerCount} {t("followers")}
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-2">
                <Link
                  href={`/p/${p.slug}`}
                  className="rounded-md border border-[var(--app-border)] px-3 py-1.5 text-xs text-[var(--app-text-secondary)] hover:border-[var(--app-border-hover)] transition-colors"
                >
                  View
                </Link>
                <Link
                  href={`/p/${p.slug}/admin`}
                  className="rounded-md border border-[#f09a45]/40 px-3 py-1.5 text-xs text-[#f09a45] hover:bg-[#f09a45]/10 transition-colors"
                >
                  {t("adminPanel")}
                </Link>
              </div>
            </div>
          ))}
        </div>
      )}

      {showCreate && (
        <CreatePageModal
          onCreated={(created) => {
            setPages((prev) => [...prev, { ...created, followerCount: 0 }]);
            setShowCreate(false);
            router.push(`/p/${created.slug}/admin`);
          }}
          onClose={() => setShowCreate(false)}
        />
      )}
    </div>
  );
}
