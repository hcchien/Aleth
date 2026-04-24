"use client";

import React, { useEffect, useState } from "react";
import { AnimatePresence, motion } from "framer-motion";

import type { User } from "@/types/auth";
import {
  type CreateTrustedIssuerRequest,
  type TrustedIssuer,
  type VerificationCase,
  type VerifierDecisionValue,
  createTrustedIssuer,
  createVerifierDecision,
  fetchTrustedIssuers,
  fetchVerifierCases,
} from "@/lib/api";

type VerifierConsoleProps = {
  user: User | null;
};

type ConsoleTab = "cases" | "issuers";
type IssuanceSource = "internal_verifier_issued" | "external_issuer_verified";

const DECISION_COPY: Record<VerifierDecisionValue, string> = {
  approve: "核准",
  reject: "拒絕",
  request_more_evidence: "補件",
};

const SOURCE_COPY: Record<IssuanceSource, string> = {
  internal_verifier_issued: "內部核發",
  external_issuer_verified: "外部 issuer 驗證",
};

const EMPTY_ISSUER_FORM: CreateTrustedIssuerRequest = {
  issuerDid: "",
  issuerName: "",
  scopes: ["professional"],
  credentialTypes: ["contribution_history"],
  maxTrustTierIssued: 3,
  status: "active",
  expiresAt: "",
};

function toDateTimeLocal(value?: string) {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  const offset = date.getTimezoneOffset();
  const localDate = new Date(date.getTime() - offset * 60_000);
  return localDate.toISOString().slice(0, 16);
}

export default function VerifierConsole({ user }: VerifierConsoleProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<ConsoleTab>("cases");
  const [cases, setCases] = useState<VerificationCase[]>([]);
  const [issuers, setIssuers] = useState<TrustedIssuer[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [activeCaseId, setActiveCaseId] = useState<string | null>(null);
  const [activeIssuerDid, setActiveIssuerDid] = useState<string | null>(null);
  const [reasonByCase, setReasonByCase] = useState<Record<string, string>>({});
  const [issuanceSourceByCase, setIssuanceSourceByCase] = useState<Record<string, IssuanceSource>>({});
  const [selectedIssuerByCase, setSelectedIssuerByCase] = useState<Record<string, string>>({});
  const [issuerForm, setIssuerForm] = useState<CreateTrustedIssuerRequest>(EMPTY_ISSUER_FORM);
  const [error, setError] = useState<string | null>(null);

  async function loadConsoleData(currentUser: User) {
    setIsLoading(true);
    setError(null);
    try {
      const [nextCases, nextIssuers] = await Promise.all([
        fetchVerifierCases(currentUser),
        fetchTrustedIssuers(currentUser),
      ]);
      setCases(nextCases);
      setIssuers(nextIssuers);
      setIssuanceSourceByCase((prev) => {
        const next = { ...prev };
        for (const verificationCase of nextCases) {
          if (!next[verificationCase.id]) {
            next[verificationCase.id] = "internal_verifier_issued";
          }
        }
        return next;
      });
    } catch (loadError) {
      console.error("Failed to load verifier console data", loadError);
      setError("無法載入 verifier console。請確認目前 DID 已被 bootstrap 成 L4 verifier。");
    } finally {
      setIsLoading(false);
    }
  }

  useEffect(() => {
    if (!isOpen || !user) {
      return;
    }
    const currentUser = user;
    let cancelled = false;
    async function run() {
      if (cancelled) {
        return;
      }
      await loadConsoleData(currentUser);
    }
    void run();
    return () => {
      cancelled = true;
    };
  }, [isOpen, user]);

  async function submitDecision(verificationCase: VerificationCase, decision: VerifierDecisionValue) {
    if (!user) {
      return;
    }
    const reason = reasonByCase[verificationCase.id]?.trim();
    const issuanceSource = issuanceSourceByCase[verificationCase.id] ?? "internal_verifier_issued";
    const externalIssuerDid = selectedIssuerByCase[verificationCase.id]?.trim();
    if (!reason) {
      setError("請先填寫 decision reason，讓 audit log 有可追溯的判斷依據。");
      return;
    }
    if (
      decision === "approve" &&
      verificationCase.requestedTier >= 3 &&
      issuanceSource === "external_issuer_verified" &&
      !externalIssuerDid
    ) {
      setError("選擇外部 issuer 驗證時，必須指定 trusted issuer。");
      return;
    }
    setActiveCaseId(verificationCase.id);
    setError(null);
    try {
      await createVerifierDecision(
        verificationCase.id,
        {
          decision,
          reason,
          credentialIssuanceSource: issuanceSource,
          externalIssuerDid: issuanceSource === "external_issuer_verified" ? externalIssuerDid : undefined,
        },
        user,
      );
      await loadConsoleData(user);
      setReasonByCase((prev) => ({ ...prev, [verificationCase.id]: "" }));
    } catch (decisionError) {
      console.error("Failed to create verifier decision", decisionError);
      setError("送出 verifier decision 失敗。");
    } finally {
      setActiveCaseId(null);
    }
  }

  async function submitIssuerForm() {
    if (!user) {
      return;
    }
    if (!issuerForm.issuerDid.trim() || !issuerForm.issuerName.trim() || issuerForm.credentialTypes.length === 0) {
      setError("請完整填寫 issuer DID、名稱與至少一個 credential type。");
      return;
    }
    setActiveIssuerDid(issuerForm.issuerDid.trim());
    setError(null);
    try {
      await createTrustedIssuer(
        {
          ...issuerForm,
          issuerDid: issuerForm.issuerDid.trim(),
          issuerName: issuerForm.issuerName.trim(),
          scopes: issuerForm.scopes.map((value) => value.trim()).filter(Boolean),
          credentialTypes: issuerForm.credentialTypes.map((value) => value.trim()).filter(Boolean),
          expiresAt: issuerForm.expiresAt?.trim() ? new Date(issuerForm.expiresAt).toISOString() : undefined,
        },
        user,
      );
      setIssuerForm(EMPTY_ISSUER_FORM);
      await loadConsoleData(user);
    } catch (issuerError) {
      console.error("Failed to create trusted issuer", issuerError);
      setError("新增 trusted issuer 失敗。");
    } finally {
      setActiveIssuerDid(null);
    }
  }

  async function setIssuerStatus(issuer: TrustedIssuer, status: TrustedIssuer["status"]) {
    if (!user) {
      return;
    }
    setActiveIssuerDid(issuer.issuerDid);
    setError(null);
    try {
      await createTrustedIssuer(
        {
          issuerDid: issuer.issuerDid,
          issuerName: issuer.issuerName,
          scopes: issuer.scopes,
          credentialTypes: issuer.credentialTypes,
          maxTrustTierIssued: issuer.maxTrustTierIssued,
          status,
          expiresAt: issuer.expiresAt,
        },
        user,
      );
      await loadConsoleData(user);
    } catch (issuerError) {
      console.error("Failed to update trusted issuer", issuerError);
      setError("更新 trusted issuer 狀態失敗。");
    } finally {
      setActiveIssuerDid(null);
    }
  }

  return (
    <>
      <button
        type="button"
        onClick={() => setIsOpen(true)}
        className="fixed bottom-6 right-6 z-40 flex items-center gap-3 rounded-full border border-red-500/30 bg-red-950/85 px-6 py-3 text-sm font-bold text-red-100 shadow-2xl shadow-red-950/30 backdrop-blur-md transition hover:scale-105 hover:bg-red-900"
      >
        <span className="h-2 w-2 rounded-full bg-red-400 shadow-[0_0_16px_rgba(248,113,113,0.9)]" />
        L4 Verifier Console
      </button>

      <AnimatePresence>
        {isOpen ? (
          <motion.div
            initial={{ opacity: 0, y: 40 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 40 }}
            className="fixed inset-0 z-50 flex items-end justify-center bg-black/65 p-4 backdrop-blur-sm sm:items-center"
            onClick={() => setIsOpen(false)}
          >
            <motion.div
              className="flex h-[88vh] w-full max-w-6xl flex-col overflow-hidden rounded-[28px] border border-neutral-800 bg-neutral-950 shadow-2xl"
              onClick={(event) => event.stopPropagation()}
            >
              <header className="flex items-center justify-between border-b border-neutral-800 bg-black/50 px-8 py-6">
                <div>
                  <div className="text-[10px] font-black uppercase tracking-[0.28em] text-red-400">Authority Desk</div>
                  <h2 className="mt-2 text-2xl font-black tracking-tight text-white">Verifier Console</h2>
                  <p className="mt-1 text-xs text-neutral-400">
                    Active signer: <span className="font-mono text-neutral-200">{user?.address ?? "unknown"}</span>
                  </p>
                </div>
                <button
                  type="button"
                  className="h-10 w-10 rounded-full bg-neutral-900 text-neutral-400 transition hover:bg-neutral-800 hover:text-white"
                  onClick={() => setIsOpen(false)}
                  aria-label="Close verifier console"
                >
                  x
                </button>
              </header>

              <div className="grid flex-1 overflow-hidden lg:grid-cols-[280px_1fr]">
                <aside className="border-b border-neutral-800 bg-neutral-950/80 p-6 lg:border-b-0 lg:border-r">
                  <div className="rounded-3xl border border-red-900/30 bg-red-950/20 p-5">
                    <div className="text-[10px] font-black uppercase tracking-[0.24em] text-red-300">L4 Boundary</div>
                    <p className="mt-3 text-sm leading-6 text-neutral-300">
                      這裡管理 L2/L3 verification queue 與 trusted issuer registry。外部 issuer 只有進入 registry 後，才能支撐
                      `external_issuer_verified` 的 L3 credential 路徑。
                    </p>
                  </div>
                  <div className="mt-6 grid grid-cols-2 gap-3 text-center">
                    <div className="rounded-2xl bg-neutral-900 p-4">
                      <div className="text-2xl font-black text-white">{cases.length}</div>
                      <div className="mt-1 text-[10px] uppercase tracking-widest text-neutral-500">Cases</div>
                    </div>
                    <div className="rounded-2xl bg-neutral-900 p-4">
                      <div className="text-2xl font-black text-red-300">{issuers.length}</div>
                      <div className="mt-1 text-[10px] uppercase tracking-widest text-neutral-500">Issuers</div>
                    </div>
                  </div>

                  <div className="mt-6 space-y-2">
                    <button
                      type="button"
                      onClick={() => setActiveTab("cases")}
                      className={
                        activeTab === "cases"
                          ? "w-full rounded-2xl bg-white px-4 py-3 text-left text-sm font-black text-neutral-950"
                          : "w-full rounded-2xl bg-neutral-900 px-4 py-3 text-left text-sm font-bold text-neutral-300 transition hover:bg-neutral-800"
                      }
                    >
                      Verification Queue
                    </button>
                    <button
                      type="button"
                      onClick={() => setActiveTab("issuers")}
                      className={
                        activeTab === "issuers"
                          ? "w-full rounded-2xl bg-white px-4 py-3 text-left text-sm font-black text-neutral-950"
                          : "w-full rounded-2xl bg-neutral-900 px-4 py-3 text-left text-sm font-bold text-neutral-300 transition hover:bg-neutral-800"
                      }
                    >
                      Trusted Issuers
                    </button>
                  </div>
                </aside>

                <main className="overflow-y-auto p-6">
                  {error ? (
                    <div className="mb-5 rounded-2xl border border-red-500/30 bg-red-950/30 px-4 py-3 text-sm text-red-100">
                      {error}
                    </div>
                  ) : null}

                  {isLoading ? (
                    <div className="flex h-full items-center justify-center text-sm text-neutral-500">Loading verifier console...</div>
                  ) : activeTab === "cases" ? (
                    cases.length === 0 ? (
                      <div className="flex h-full items-center justify-center">
                        <div className="max-w-sm text-center">
                          <div className="text-[10px] font-black uppercase tracking-[0.28em] text-neutral-600">Queue Clear</div>
                          <p className="mt-3 text-sm leading-6 text-neutral-400">目前沒有待審核的 verification case。</p>
                        </div>
                      </div>
                    ) : (
                      <div className="space-y-5">
                        {cases.map((verificationCase) => {
                          const isBusy = activeCaseId === verificationCase.id;
                          const issuanceSource = issuanceSourceByCase[verificationCase.id] ?? "internal_verifier_issued";
                          const eligibleIssuers = issuers.filter(
                            (issuer) =>
                              issuer.status === "active" &&
                              issuer.maxTrustTierIssued >= verificationCase.requestedTier &&
                              issuer.credentialTypes.includes(verificationCase.credentialType),
                          );
                          return (
                            <article key={verificationCase.id} className="rounded-3xl border border-neutral-800 bg-neutral-900/70 p-6">
                              <div className="flex flex-col justify-between gap-4 md:flex-row">
                                <div>
                                  <div className="font-mono text-[11px] uppercase tracking-wider text-neutral-500">{verificationCase.id}</div>
                                  <h3 className="mt-2 text-xl font-black text-white">
                                    L{verificationCase.requestedTier} · {verificationCase.credentialType}
                                  </h3>
                                  <p className="mt-1 text-xs text-neutral-400">
                                    Subject: <span className="font-mono text-neutral-200">{verificationCase.subjectDid}</span>
                                  </p>
                                </div>
                                <span className="h-fit rounded-full border border-neutral-700 px-3 py-1 text-xs font-bold uppercase tracking-wider text-neutral-300">
                                  {verificationCase.status}
                                </span>
                              </div>

                              <pre className="mt-5 max-h-40 overflow-auto rounded-2xl border border-neutral-800 bg-black/40 p-4 text-xs leading-6 text-neutral-300">
                                {verificationCase.evidenceJson}
                              </pre>

                              {verificationCase.requestedTier >= 3 ? (
                                <div className="mt-5 grid gap-3 lg:grid-cols-[220px_1fr]">
                                  <div>
                                    <label className="mb-2 block text-[10px] font-black uppercase tracking-[0.24em] text-neutral-500">
                                      Issuance Source
                                    </label>
                                    <select
                                      value={issuanceSource}
                                      onChange={(event) =>
                                        setIssuanceSourceByCase((prev) => ({
                                          ...prev,
                                          [verificationCase.id]: event.target.value as IssuanceSource,
                                        }))
                                      }
                                      className="w-full rounded-2xl border border-neutral-800 bg-neutral-950 px-4 py-3 text-sm text-white outline-none"
                                    >
                                      <option value="internal_verifier_issued">{SOURCE_COPY.internal_verifier_issued}</option>
                                      <option value="external_issuer_verified">{SOURCE_COPY.external_issuer_verified}</option>
                                    </select>
                                  </div>

                                  {issuanceSource === "external_issuer_verified" ? (
                                    <div>
                                      <label className="mb-2 block text-[10px] font-black uppercase tracking-[0.24em] text-neutral-500">
                                        Trusted Issuer
                                      </label>
                                      <select
                                        value={selectedIssuerByCase[verificationCase.id] ?? ""}
                                        onChange={(event) =>
                                          setSelectedIssuerByCase((prev) => ({
                                            ...prev,
                                            [verificationCase.id]: event.target.value,
                                          }))
                                        }
                                        className="w-full rounded-2xl border border-neutral-800 bg-neutral-950 px-4 py-3 text-sm text-white outline-none"
                                      >
                                        <option value="">Select issuer</option>
                                        {eligibleIssuers.map((issuer) => (
                                          <option key={issuer.issuerDid} value={issuer.issuerDid}>
                                            {issuer.issuerName} · {issuer.issuerDid}
                                          </option>
                                        ))}
                                      </select>
                                    </div>
                                  ) : null}
                                </div>
                              ) : null}

                              <textarea
                                value={reasonByCase[verificationCase.id] ?? ""}
                                onChange={(event) =>
                                  setReasonByCase((prev) => ({ ...prev, [verificationCase.id]: event.target.value }))
                                }
                                placeholder="Decision reason..."
                                className="mt-5 min-h-24 w-full rounded-2xl border border-neutral-800 bg-neutral-950 px-4 py-3 text-sm leading-6 text-white outline-none transition focus:border-red-400/50"
                              />

                              <div className="mt-5 grid gap-3 md:grid-cols-3">
                                {(["approve", "request_more_evidence", "reject"] as VerifierDecisionValue[]).map((decision) => (
                                  <button
                                    key={decision}
                                    type="button"
                                    disabled={isBusy || verificationCase.status === "approved" || verificationCase.status === "rejected"}
                                    onClick={() => void submitDecision(verificationCase, decision)}
                                    className={
                                      decision === "approve"
                                        ? "rounded-2xl bg-emerald-500 px-4 py-3 text-sm font-black text-emerald-950 transition hover:bg-emerald-400 disabled:opacity-40"
                                        : decision === "reject"
                                          ? "rounded-2xl bg-red-600 px-4 py-3 text-sm font-black text-white transition hover:bg-red-500 disabled:opacity-40"
                                          : "rounded-2xl bg-neutral-800 px-4 py-3 text-sm font-black text-neutral-100 transition hover:bg-neutral-700 disabled:opacity-40"
                                    }
                                  >
                                    {isBusy ? "處理中..." : DECISION_COPY[decision]}
                                  </button>
                                ))}
                              </div>
                            </article>
                          );
                        })}
                      </div>
                    )
                  ) : (
                    <div className="grid gap-6 xl:grid-cols-[360px_1fr]">
                      <section className="rounded-3xl border border-neutral-800 bg-neutral-900/70 p-6">
                        <div className="text-[10px] font-black uppercase tracking-[0.24em] text-neutral-500">Register Issuer</div>
                        <h3 className="mt-2 text-xl font-black text-white">新增 Trusted Issuer</h3>
                        <div className="mt-5 space-y-4">
                          <input
                            value={issuerForm.issuerDid}
                            onChange={(event) => setIssuerForm((prev) => ({ ...prev, issuerDid: event.target.value }))}
                            placeholder="did:web:issuer.example"
                            className="w-full rounded-2xl border border-neutral-800 bg-neutral-950 px-4 py-3 text-sm text-white outline-none"
                          />
                          <input
                            value={issuerForm.issuerName}
                            onChange={(event) => setIssuerForm((prev) => ({ ...prev, issuerName: event.target.value }))}
                            placeholder="Issuer name"
                            className="w-full rounded-2xl border border-neutral-800 bg-neutral-950 px-4 py-3 text-sm text-white outline-none"
                          />
                          <input
                            value={issuerForm.scopes.join(", ")}
                            onChange={(event) =>
                              setIssuerForm((prev) => ({
                                ...prev,
                                scopes: event.target.value.split(",").map((value) => value.trim()).filter(Boolean),
                              }))
                            }
                            placeholder="professional, community"
                            className="w-full rounded-2xl border border-neutral-800 bg-neutral-950 px-4 py-3 text-sm text-white outline-none"
                          />
                          <input
                            value={issuerForm.credentialTypes.join(", ")}
                            onChange={(event) =>
                              setIssuerForm((prev) => ({
                                ...prev,
                                credentialTypes: event.target.value.split(",").map((value) => value.trim()).filter(Boolean),
                              }))
                            }
                            placeholder="contribution_history"
                            className="w-full rounded-2xl border border-neutral-800 bg-neutral-950 px-4 py-3 text-sm text-white outline-none"
                          />
                          <div className="grid grid-cols-2 gap-3">
                            <select
                              value={issuerForm.maxTrustTierIssued}
                              onChange={(event) =>
                                setIssuerForm((prev) => ({ ...prev, maxTrustTierIssued: Number(event.target.value) }))
                              }
                              className="w-full rounded-2xl border border-neutral-800 bg-neutral-950 px-4 py-3 text-sm text-white outline-none"
                            >
                              <option value={2}>Max L2</option>
                              <option value={3}>Max L3</option>
                              <option value={4}>Max L4</option>
                            </select>
                            <select
                              value={issuerForm.status}
                              onChange={(event) =>
                                setIssuerForm((prev) => ({
                                  ...prev,
                                  status: event.target.value as NonNullable<CreateTrustedIssuerRequest["status"]>,
                                }))
                              }
                              className="w-full rounded-2xl border border-neutral-800 bg-neutral-950 px-4 py-3 text-sm text-white outline-none"
                            >
                              <option value="active">Active</option>
                              <option value="suspended">Suspended</option>
                              <option value="expired">Expired</option>
                            </select>
                          </div>
                          <input
                            type="datetime-local"
                            value={issuerForm.expiresAt ?? ""}
                            onChange={(event) => setIssuerForm((prev) => ({ ...prev, expiresAt: event.target.value }))}
                            className="w-full rounded-2xl border border-neutral-800 bg-neutral-950 px-4 py-3 text-sm text-white outline-none"
                          />
                          <button
                            type="button"
                            disabled={activeIssuerDid === issuerForm.issuerDid.trim()}
                            onClick={() => void submitIssuerForm()}
                            className="w-full rounded-2xl bg-white px-4 py-3 text-sm font-black text-neutral-950 transition hover:bg-neutral-200 disabled:opacity-50"
                          >
                            {activeIssuerDid === issuerForm.issuerDid.trim() ? "寫入中..." : "新增 / 更新 Issuer"}
                          </button>
                        </div>
                      </section>

                      <section className="space-y-4">
                        {issuers.length === 0 ? (
                          <div className="rounded-3xl border border-neutral-800 bg-neutral-900/70 p-6 text-sm text-neutral-400">
                            尚未註冊 trusted issuer。
                          </div>
                        ) : (
                          issuers.map((issuer) => {
                            const isBusy = activeIssuerDid === issuer.issuerDid;
                            return (
                              <article key={issuer.issuerDid} className="rounded-3xl border border-neutral-800 bg-neutral-900/70 p-6">
                                <div className="flex flex-col justify-between gap-4 md:flex-row">
                                  <div>
                                    <div className="font-mono text-[11px] uppercase tracking-wider text-neutral-500">{issuer.issuerDid}</div>
                                    <h3 className="mt-2 text-xl font-black text-white">{issuer.issuerName}</h3>
                                    <p className="mt-2 text-xs text-neutral-400">
                                      Types: {issuer.credentialTypes.join(", ")} · Max Tier L{issuer.maxTrustTierIssued}
                                    </p>
                                    <p className="mt-1 text-xs text-neutral-500">Scopes: {issuer.scopes.join(", ") || "none"}</p>
                                    <p className="mt-1 text-xs text-neutral-500">
                                      Expires: {issuer.expiresAt ? new Date(issuer.expiresAt).toLocaleString() : "none"}
                                    </p>
                                    <p className="mt-1 text-xs text-neutral-500">
                                      Revoked: {issuer.revokedAt ? new Date(issuer.revokedAt).toLocaleString() : "not revoked"}
                                    </p>
                                  </div>
                                  <div className="flex flex-col items-start gap-3 md:items-end">
                                    <span
                                      className={
                                        issuer.status === "active"
                                          ? "rounded-full bg-emerald-500/15 px-3 py-1 text-xs font-black uppercase tracking-wider text-emerald-300"
                                          : "rounded-full bg-neutral-700 px-3 py-1 text-xs font-black uppercase tracking-wider text-neutral-300"
                                      }
                                    >
                                      {issuer.status}
                                    </span>
                                    <div className="flex gap-2">
                                      {issuer.status !== "revoked" ? (
                                        <>
                                          {issuer.status === "active" ? (
                                            <button
                                              type="button"
                                              disabled={isBusy}
                                              onClick={() => void setIssuerStatus(issuer, "suspended")}
                                              className="rounded-2xl bg-neutral-700 px-4 py-2 text-xs font-black text-white transition hover:bg-neutral-600 disabled:opacity-40"
                                            >
                                              {isBusy ? "更新中..." : "停用"}
                                            </button>
                                          ) : (
                                            <button
                                              type="button"
                                              disabled={isBusy}
                                              onClick={() => void setIssuerStatus(issuer, "active")}
                                              className="rounded-2xl bg-emerald-500 px-4 py-2 text-xs font-black text-emerald-950 transition hover:bg-emerald-400 disabled:opacity-40"
                                            >
                                              {isBusy ? "更新中..." : "重新啟用"}
                                            </button>
                                          )}
                                          <button
                                            type="button"
                                            disabled={isBusy}
                                            onClick={() => void setIssuerStatus(issuer, "revoked")}
                                            className="rounded-2xl bg-red-600 px-4 py-2 text-xs font-black text-white transition hover:bg-red-500 disabled:opacity-40"
                                          >
                                            {isBusy ? "更新中..." : "Revoke"}
                                          </button>
                                        </>
                                      ) : null}
                                    </div>
                                    {issuer.status !== "revoked" ? (
                                      <button
                                        type="button"
                                        disabled={isBusy}
                                        onClick={() =>
                                          setIssuerForm({
                                            issuerDid: issuer.issuerDid,
                                            issuerName: issuer.issuerName,
                                            scopes: issuer.scopes,
                                            credentialTypes: issuer.credentialTypes,
                                            maxTrustTierIssued: issuer.maxTrustTierIssued,
                                            status: issuer.status,
                                            expiresAt: toDateTimeLocal(issuer.expiresAt),
                                          })
                                        }
                                        className="text-xs font-bold text-neutral-400 transition hover:text-white"
                                      >
                                        Edit
                                      </button>
                                    ) : null}
                                  </div>
                                </div>
                              </article>
                            );
                          })
                        )}
                      </section>
                    </div>
                  )}
                </main>
              </div>
            </motion.div>
          </motion.div>
        ) : null}
      </AnimatePresence>
    </>
  );
}
