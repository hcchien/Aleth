"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { gqlClient } from "@/lib/gql-client";

const AVAILABLE_VC_TYPES_QUERY = `
  query {
    availableVcTypes {
      vcType
      issuer
      label
      description
      createdByUsername
    }
  }
`;

const SET_PAGE_POLICY = `
  mutation SetPagePolicy($pageId: ID!, $input: PagePolicyInput!) {
    setPagePolicy(pageId: $pageId, input: $input) {
      id
      defaultAccess
      minTrustLevel
      commentPolicy
      minCommentTrust
      requireVcs { vcType issuer }
      requireCommentVcs { vcType issuer }
    }
  }
`;

export interface VcRequirement {
  vcType: string;
  issuer: string;
}

export interface PagePolicy {
  defaultAccess: string;
  minTrustLevel: number;
  commentPolicy: string;
  minCommentTrust: number;
  requireVcs: VcRequirement[];
  requireCommentVcs: VcRequirement[];
}

interface VcTypeInfo {
  vcType: string;
  issuer: string;
  label: string;
  description: string | null;
  createdByUsername: string | null;
}

function vcKey(r: VcRequirement) {
  return `${r.vcType}::${r.issuer}`;
}

function VcMultiSelect({
  label,
  selected,
  registry,
  onChange,
}: {
  label: string;
  selected: VcRequirement[];
  registry: VcTypeInfo[];
  onChange: (next: VcRequirement[]) => void;
}) {
  const t = useTranslations("boardPolicy");
  const selectedKeys = new Set(selected.map(vcKey));

  function toggle(entry: VcTypeInfo) {
    const key = vcKey(entry);
    if (selectedKeys.has(key)) {
      onChange(selected.filter((r) => vcKey(r) !== key));
    } else {
      onChange([...selected, { vcType: entry.vcType, issuer: entry.issuer }]);
    }
  }

  const grouped = registry.reduce<Record<string, VcTypeInfo[]>>((acc, e) => {
    (acc[e.issuer] ??= []).push(e);
    return acc;
  }, {});

  return (
    <div className="space-y-2">
      <p className="text-xs font-medium uppercase tracking-wide text-[#7a8090]">
        {label}
      </p>
      {registry.length === 0 && (
        <p className="text-xs italic text-[#7a8090]">{t("noVcTypes")}</p>
      )}
      {Object.entries(grouped).map(([issuer, entries]) => (
        <div key={issuer} className="rounded-lg border border-[#1e2230] bg-[#0c0f17] p-3">
          <p className="mb-2 text-[10px] font-semibold uppercase tracking-wider text-[#555c6e]">
            Issuer: {issuer === "platform" ? t("issuerPlatform") : `@${issuer}`}
          </p>
          <div className="space-y-1.5">
            {entries.map((e) => {
              const key = vcKey(e);
              const checked = selectedKeys.has(key);
              return (
                <label
                  key={key}
                  className="flex cursor-pointer items-start gap-3 rounded-md px-2 py-1.5 transition-colors hover:bg-[#151820]"
                >
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() => toggle(e)}
                    className="mt-0.5 h-3.5 w-3.5 shrink-0 accent-[#f09a45]"
                  />
                  <div className="min-w-0">
                    <span className="text-sm text-[#e6e7ea]">{e.label}</span>
                    {e.description && (
                      <p className="mt-0.5 text-xs text-[#7a8090]">{e.description}</p>
                    )}
                    <p className="text-[10px] text-[#555c6e] font-mono">{e.vcType}</p>
                  </div>
                </label>
              );
            })}
          </div>
        </div>
      ))}
    </div>
  );
}

interface Props {
  pageId: string;
  initial: PagePolicy;
}

export function PagePolicyForm({ pageId, initial }: Props) {
  const t = useTranslations("fanPage");
  const tp = useTranslations("boardPolicy");
  const tCommon = useTranslations("common");
  const [registry, setRegistry] = useState<VcTypeInfo[]>([]);
  const [registryLoading, setRegistryLoading] = useState(true);

  const [defaultAccess, setDefaultAccess] = useState(initial.defaultAccess);
  const [minTrustLevel, setMinTrustLevel] = useState(initial.minTrustLevel);
  const [commentPolicy, setCommentPolicy] = useState(initial.commentPolicy);
  const [minCommentTrust, setMinCommentTrust] = useState(initial.minCommentTrust);
  const [requireVcs, setRequireVcs] = useState<VcRequirement[]>(initial.requireVcs);
  const [requireCommentVcs, setRequireCommentVcs] = useState<VcRequirement[]>(initial.requireCommentVcs);

  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    gqlClient<{ availableVcTypes: VcTypeInfo[] }>(AVAILABLE_VC_TYPES_QUERY)
      .then((data) => setRegistry(data.availableVcTypes))
      .catch(() => {})
      .finally(() => setRegistryLoading(false));
  }, []);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    setSaved(false);
    try {
      await gqlClient(SET_PAGE_POLICY, {
        pageId,
        input: {
          defaultAccess,
          minTrustLevel,
          commentPolicy,
          minCommentTrust,
          requireVcs,
          requireCommentVcs,
        },
      });
      setSaved(true);
      setTimeout(() => setSaved(false), 3000);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed");
    } finally {
      setSaving(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-8">
      {/* ── Post access policy ── */}
      <section className="rounded-2xl border border-[#2a2e38] bg-[#0f1117] p-6 space-y-5">
        <h2 className="font-semibold text-[#f3f5f9]">{t("policySection")}</h2>
        <p className="text-xs text-[#7a8090]">{tp("articlePolicyDesc")}</p>

        <div>
          <label className="mb-1.5 block text-xs font-medium uppercase tracking-wide text-[#7a8090]">
            {t("defaultAccess")}
          </label>
          <select
            value={defaultAccess}
            onChange={(e) => setDefaultAccess(e.target.value)}
            className="w-full rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-2 text-sm text-[#e6e7ea] focus:border-[#f09a45] focus:outline-none"
          >
            <option value="public">{t("accessPublic")}</option>
            <option value="members">{t("accessMembers")}</option>
          </select>
        </div>

        <div>
          <label className="mb-1.5 block text-xs font-medium uppercase tracking-wide text-[#7a8090]">
            {tp("minTrustLevel")}
          </label>
          <select
            value={minTrustLevel}
            onChange={(e) => setMinTrustLevel(Number(e.target.value))}
            className="w-full rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-2 text-sm text-[#e6e7ea] focus:border-[#f09a45] focus:outline-none"
          >
            {[0, 1, 2, 3, 4].map((lvl) => (
              <option key={lvl} value={lvl}>{tp(`trustLevels.${lvl}`)}</option>
            ))}
          </select>
        </div>

        {registryLoading ? (
          <p className="text-xs text-[#7a8090]">{tCommon("loading")}</p>
        ) : (
          <VcMultiSelect
            label={tp("requiredVcs")}
            selected={requireVcs}
            registry={registry}
            onChange={setRequireVcs}
          />
        )}
      </section>

      {/* ── Comment policy ── */}
      <section className="rounded-2xl border border-[#2a2e38] bg-[#0f1117] p-6 space-y-5">
        <h2 className="font-semibold text-[#f3f5f9]">{tp("commentPolicy")}</h2>
        <p className="text-xs text-[#7a8090]">{tp("commentPolicyDesc")}</p>

        <div>
          <label className="mb-1.5 block text-xs font-medium uppercase tracking-wide text-[#7a8090]">
            {t("commentPolicy")}
          </label>
          <select
            value={commentPolicy}
            onChange={(e) => setCommentPolicy(e.target.value)}
            className="w-full rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-2 text-sm text-[#e6e7ea] focus:border-[#f09a45] focus:outline-none"
          >
            <option value="public">{t("commentPublic")}</option>
            <option value="members">{t("commentMembers")}</option>
          </select>
        </div>

        <div>
          <label className="mb-1.5 block text-xs font-medium uppercase tracking-wide text-[#7a8090]">
            {tp("minCommentTrust")}
          </label>
          <select
            value={minCommentTrust}
            onChange={(e) => setMinCommentTrust(Number(e.target.value))}
            className="w-full rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-2 text-sm text-[#e6e7ea] focus:border-[#f09a45] focus:outline-none"
          >
            {[0, 1, 2, 3, 4].map((lvl) => (
              <option key={lvl} value={lvl}>{tp(`trustLevels.${lvl}`)}</option>
            ))}
          </select>
        </div>

        {!registryLoading && (
          <VcMultiSelect
            label={tp("requiredCommentVcs")}
            selected={requireCommentVcs}
            registry={registry}
            onChange={setRequireCommentVcs}
          />
        )}
      </section>

      {/* ── Actions ── */}
      <div className="flex items-center gap-4">
        <button
          type="submit"
          disabled={saving}
          className="rounded-md bg-[#f09a45] px-5 py-2 text-sm font-medium text-[#0b0d12] hover:bg-[#fbb468] disabled:opacity-50 transition-colors"
        >
          {saving ? tp("saving") : tp("savePolicy")}
        </button>
        {saved && <span className="text-sm text-emerald-400">{tp("saved")}</span>}
        {error && <span className="text-sm text-red-400">{error}</span>}
      </div>
    </form>
  );
}
