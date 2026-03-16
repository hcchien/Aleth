"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useAuth } from "@/lib/auth-context";
import { gqlClient } from "@/lib/gql-client";
import { BoardPolicyForm, type BoardPolicy } from "@/app/components/board-policy-form";

const BOARD_POLICY_QUERY = `
  query BoardPolicy($username: String!) {
    board(username: $username) {
      id
      name
      writePolicy {
        minTrustLevel
        requireVcs { vcType issuer }
        minCommentTrust
        requireCommentVcs { vcType issuer }
      }
    }
  }
`;

interface BoardData {
  id: string;
  name: string;
  writePolicy: BoardPolicy;
}

export default function BoardSettingsPage() {
  const t = useTranslations("boardSettings");
  const tp = useTranslations("boardPolicy");
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();
  const [board, setBoard] = useState<BoardData | null>(null);
  const [fetching, setFetching] = useState(true);
  const [fetchError, setFetchError] = useState<string | null>(null);

  useEffect(() => {
    if (authLoading) return;
    if (!user) {
      router.replace("/login?redirect=/settings/board");
      return;
    }

    gqlClient<{ board: BoardData | null }>(BOARD_POLICY_QUERY, {
      username: user.username,
    })
      .then((data) => {
        setBoard(data.board);
      })
      .catch((err) => {
        setFetchError(err instanceof Error ? err.message : t("loadFailed"));
      })
      .finally(() => setFetching(false));
  }, [user, authLoading]);

  if (authLoading || fetching) {
    return (
      <div className="mx-auto mt-10 max-w-2xl px-4 text-sm text-[#7a8090]">
        Loading…
      </div>
    );
  }

  return (
    <div className="mx-auto mt-10 max-w-2xl px-4 pb-20">
      {/* Breadcrumb */}
      <nav className="mb-6 flex items-center gap-2 text-sm text-[#7a8090]">
        <Link href="/" className="hover:text-[#c8cdd8] transition-colors">
          {t("breadcrumbHome")}
        </Link>
        <span>›</span>
        <Link
          href={`/@${user?.username}`}
          className="hover:text-[#c8cdd8] transition-colors"
        >
          {t("breadcrumbBoard")}
        </Link>
        <span>›</span>
        <span className="text-[#c8cdd8]">{t("breadcrumbPolicy")}</span>
      </nav>

      <h1 className="mb-1 font-serif text-2xl text-[#f3f5f9]">
        {t("title")}
      </h1>
      {board && (
        <p className="mb-8 text-sm text-[#7a8090]">
          {t("configuringBoard", { boardName: board.name })}
        </p>
      )}

      {fetchError && (
        <div className="mb-6 rounded-xl border border-red-900/50 bg-red-950/30 px-4 py-3 text-sm text-red-400">
          {fetchError}
        </div>
      )}

      {!board && !fetchError && (
        <div className="rounded-xl border border-[#2a2e38] bg-[#0f1117] px-6 py-10 text-center text-sm text-[#7a8090]">
          {t("noBoard")}{" "}
          <Link href="/" className="text-[#f09a45] hover:underline">
            {t("createFirst")}
          </Link>
        </div>
      )}

      {board && (
        <BoardPolicyForm
          initial={board.writePolicy}
        />
      )}

      {/* Trust level legend */}
      <aside className="mt-10 rounded-xl border border-[#1e2230] bg-[#0c0f17] px-5 py-4">
        <h3 className="mb-3 text-xs font-semibold uppercase tracking-wide text-[#7a8090]">
          {tp("trustLegendTitle")}
        </h3>
        <dl className="space-y-1.5 text-xs text-[#9ea4b0]">
          {[
            ["L0", tp("trustLegendL0")],
            ["L1", tp("trustLegendL1")],
            ["L2", tp("trustLegendL2")],
            ["L3", tp("trustLegendL3")],
            ["L4", tp("trustLegendL4")],
          ].map(([lvl, desc]) => (
            <div key={lvl} className="flex gap-3">
              <dt className="w-6 shrink-0 font-mono text-[#f09a45]">{lvl}</dt>
              <dd>{desc}</dd>
            </div>
          ))}
        </dl>
        <p className="mt-4 text-xs text-[#555c6e]">
          {tp("vcNote")}
        </p>
      </aside>
    </div>
  );
}
