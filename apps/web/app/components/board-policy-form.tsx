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

const UPDATE_BOARD_VC_POLICY = `
  mutation UpdateBoardVcPolicy($input: BoardVcPolicyInput!) {
    updateBoardVcPolicy(input: $input) {
      id
      writePolicy {
        minTrustLevel
        requireVcs { vcType issuer }
        minCommentTrust
        requireCommentVcs { vcType issuer }
      }
    }
  }
`;

const REGISTER_VC_TYPE_MUTATION = `
  mutation RegisterVcType($input: RegisterVcTypeInput!) {
    registerVcType(input: $input) {
      vcType
      issuer
      label
      description
      createdByUsername
    }
  }
`;

export interface VcRequirement {
  vcType: string;
  issuer: string;
}

export interface BoardPolicy {
  minTrustLevel: number;
  requireVcs: VcRequirement[];
  minCommentTrust: number;
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

// ── VC multi-select using registry ───────────────────────────────────────────

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

  // Group by issuer
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
                    <p className="text-[10px] text-[#555c6e] font-mono">
                      {e.vcType}
                    </p>
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

// ── Register new VC type inline ───────────────────────────────────────────────

function RegisterVcTypeForm({ onRegistered }: { onRegistered: (entry: VcTypeInfo) => void }) {
  const t = useTranslations("boardPolicy");
  const tCommon = useTranslations("common");
  const [open, setOpen] = useState(false);
  const [vcType, setVcType] = useState("");
  const [vcLabel, setVcLabel] = useState("");
  const [description, setDescription] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!vcType.trim() || !vcLabel.trim()) return;
    setSaving(true);
    setError(null);
    try {
      const data = await gqlClient<{ registerVcType: VcTypeInfo }>(REGISTER_VC_TYPE_MUTATION, {
        input: {
          vcType: vcType.trim(),
          label: vcLabel.trim(),
          description: description.trim() || null,
        },
      });
      onRegistered(data.registerVcType);
      setVcType("");
      setVcLabel("");
      setDescription("");
      setOpen(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to register");
    } finally {
      setSaving(false);
    }
  }

  if (!open) {
    return (
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="text-xs text-[#f09a45] hover:text-[#fbb468] transition-colors"
      >
        {t("registerVcType")}
      </button>
    );
  }

  return (
    <div className="rounded-lg border border-[#f09a45]/30 bg-[#0c0f17] p-4 space-y-3">
      <p className="text-xs font-semibold text-[#f09a45]">{t("registerVcTypeTitle")}</p>
      <p className="text-xs text-[#7a8090]">
        {t("registerVcTypeDesc")}
      </p>
      <input
        value={vcType}
        onChange={(e) => setVcType(e.target.value)}
        placeholder={t("vcTypePlaceholder")}
        className="w-full rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-1.5 text-sm text-[#e6e7ea] placeholder-[#555c6e] focus:border-[#f09a45] focus:outline-none font-mono"
      />
      <input
        value={vcLabel}
        onChange={(e) => setVcLabel(e.target.value)}
        placeholder={t("vcLabelPlaceholder")}
        className="w-full rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-1.5 text-sm text-[#e6e7ea] placeholder-[#555c6e] focus:border-[#f09a45] focus:outline-none"
      />
      <input
        value={description}
        onChange={(e) => setDescription(e.target.value)}
        placeholder={t("descriptionPlaceholder")}
        className="w-full rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-1.5 text-sm text-[#e6e7ea] placeholder-[#555c6e] focus:border-[#f09a45] focus:outline-none"
      />
      {error && <p className="text-xs text-red-400">{error}</p>}
      <div className="flex gap-2">
        <button
          type="submit"
          form="register-vc-form"
          disabled={saving || !vcType.trim() || !vcLabel.trim()}
          onClick={handleSubmit}
          className="rounded-md bg-[#f09a45] px-3 py-1.5 text-xs font-medium text-[#0b0d12] hover:bg-[#fbb468] disabled:opacity-50 transition-colors"
        >
          {saving ? t("registering") : t("register")}
        </button>
        <button
          type="button"
          onClick={() => setOpen(false)}
          className="text-xs text-[#7a8090] hover:text-[#c8cdd8] transition-colors"
        >
          {tCommon("cancel")}
        </button>
      </div>
    </div>
  );
}

// ── Main form ─────────────────────────────────────────────────────────────────

interface Props {
  initial: BoardPolicy;
}

export function BoardPolicyForm({ initial }: Props) {
  const t = useTranslations("boardPolicy");
  const [registry, setRegistry] = useState<VcTypeInfo[]>([]);
  const [registryLoading, setRegistryLoading] = useState(true);

  const [minTrustLevel, setMinTrustLevel] = useState(initial.minTrustLevel);
  const [requireVcs, setRequireVcs] = useState<VcRequirement[]>(initial.requireVcs);
  const [minCommentTrust, setMinCommentTrust] = useState(initial.minCommentTrust);
  const [requireCommentVcs, setRequireCommentVcs] = useState<VcRequirement[]>(initial.requireCommentVcs);

  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    gqlClient<{ availableVcTypes: VcTypeInfo[] }>(AVAILABLE_VC_TYPES_QUERY)
      .then((data) => setRegistry(data.availableVcTypes))
      .catch(() => {/* best-effort; show empty state */})
      .finally(() => setRegistryLoading(false));
  }, []);

  function addToRegistry(entry: VcTypeInfo) {
    setRegistry((prev) => {
      const key = vcKey(entry);
      if (prev.some((e) => vcKey(e) === key)) return prev;
      return [...prev, entry];
    });
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    setSaved(false);
    try {
      await gqlClient(UPDATE_BOARD_VC_POLICY, {
        input: {
          minTrustLevel,
          requireVcs,
          minCommentTrust,
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
      {/* ── Article write policy ── */}
      <section className="rounded-2xl border border-[#2a2e38] bg-[#0f1117] p-6 space-y-5">
        <h2 className="font-semibold text-[#f3f5f9]">{t("articlePolicy")}</h2>
        <p className="text-xs text-[#7a8090]">
          {t("articlePolicyDesc")}
        </p>

        <div>
          <label className="mb-1.5 block text-xs font-medium uppercase tracking-wide text-[#7a8090]">
            {t("minTrustLevel")}
          </label>
          <select
            value={minTrustLevel}
            onChange={(e) => setMinTrustLevel(Number(e.target.value))}
            className="w-full rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-2 text-sm text-[#e6e7ea] focus:border-[#f09a45] focus:outline-none"
          >
            {[0, 1, 2, 3, 4].map((lvl) => (
              <option key={lvl} value={lvl}>{t(`trustLevels.${lvl}`)}</option>
            ))}
          </select>
        </div>

        {registryLoading ? (
          <p className="text-xs text-[#7a8090]">Loading…</p>
        ) : (
          <>
            <VcMultiSelect
              label={t("requiredVcs")}
              selected={requireVcs}
              registry={registry}
              onChange={setRequireVcs}
            />
            <RegisterVcTypeForm onRegistered={addToRegistry} />
          </>
        )}
      </section>

      {/* ── Comment write policy ── */}
      <section className="rounded-2xl border border-[#2a2e38] bg-[#0f1117] p-6 space-y-5">
        <h2 className="font-semibold text-[#f3f5f9]">{t("commentPolicy")}</h2>
        <p className="text-xs text-[#7a8090]">
          {t("commentPolicyDesc")}
        </p>

        <div>
          <label className="mb-1.5 block text-xs font-medium uppercase tracking-wide text-[#7a8090]">
            {t("minCommentTrust")}
          </label>
          <select
            value={minCommentTrust}
            onChange={(e) => setMinCommentTrust(Number(e.target.value))}
            className="w-full rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-2 text-sm text-[#e6e7ea] focus:border-[#f09a45] focus:outline-none"
          >
            {[0, 1, 2, 3, 4].map((lvl) => (
              <option key={lvl} value={lvl}>{t(`trustLevels.${lvl}`)}</option>
            ))}
          </select>
        </div>

        {!registryLoading && (
          <VcMultiSelect
            label={t("requiredCommentVcs")}
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
          {saving ? t("saving") : t("savePolicy")}
        </button>
        {saved && <span className="text-sm text-emerald-400">{t("saved")}</span>}
        {error && <span className="text-sm text-red-400">{error}</span>}
      </div>
    </form>
  );
}
