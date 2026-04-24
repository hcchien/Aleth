"use client";

import React, { useEffect, useState, useTransition } from "react";

import type { User } from "@/types/auth";
import {
  type CreateWalletPresentationRequest as CreateWalletPresentationRequestBody,
  type TrustedIssuer,
  type WalletPresentationRequest,
  completeWalletPresentationRequest,
  createWalletPresentationRequest,
  fetchTrustedIssuers,
  fetchWalletPresentationRequests,
} from "@/lib/api";

type WalletVerificationPanelProps = {
  user: User | null;
  sessionTier: number;
};

type CompletionDraft = {
  issuerDid: string;
  claimsJson: string;
  proof: string;
};

const DEFAULT_REQUEST_FORM: CreateWalletPresentationRequestBody = {
  requestedTier: 3,
  credentialType: "organization_membership",
  purpose: "Verify issuer-backed eligibility for Aleth participation.",
  allowedIssuerDids: [],
  expiresInMinutes: 15,
};

export default function WalletVerificationPanel({ user, sessionTier }: WalletVerificationPanelProps) {
  const [requests, setRequests] = useState<WalletPresentationRequest[]>([]);
  const [issuers, setIssuers] = useState<TrustedIssuer[]>([]);
  const [requestForm, setRequestForm] = useState(DEFAULT_REQUEST_FORM);
  const [allowedIssuerInput, setAllowedIssuerInput] = useState("");
  const [completionDrafts, setCompletionDrafts] = useState<Record<string, CompletionDraft>>({});
  const [error, setError] = useState<string | null>(null);
  const [isPending, startTransition] = useTransition();

  useEffect(() => {
    if (!user) {
      return;
    }
    let cancelled = false;
    async function load() {
      try {
        const [nextRequests, nextIssuers] = await Promise.all([
          fetchWalletPresentationRequests(user),
          fetchTrustedIssuers(user),
        ]);
        if (cancelled) {
          return;
        }
        setRequests(nextRequests);
        setIssuers(nextIssuers);
        setCompletionDrafts((prev) => {
          const next = { ...prev };
          for (const request of nextRequests) {
            if (!next[request.id]) {
              next[request.id] = {
                issuerDid: request.allowedIssuerDids[0] ?? "",
                claimsJson: `{"subjectDid":"${request.subjectDid}","evidence":"prototype wallet presentation"}`,
                proof: "wallet-signature-placeholder",
              };
            }
          }
          return next;
        });
      } catch (loadError) {
        console.error("Failed to load wallet verification panel", loadError);
        if (!cancelled) {
          setError("無法載入 wallet verification 狀態。");
        }
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, [user]);

  async function refresh() {
    if (!user) {
      return;
    }
    const [nextRequests, nextIssuers] = await Promise.all([
      fetchWalletPresentationRequests(user),
      fetchTrustedIssuers(user),
    ]);
    setRequests(nextRequests);
    setIssuers(nextIssuers);
  }

  function updateCompletionDraft(requestId: string, patch: Partial<CompletionDraft>) {
    setCompletionDrafts((prev) => {
      const current = prev[requestId] ?? {
        issuerDid: "",
        claimsJson: "",
        proof: "",
      };
      return {
        ...prev,
        [requestId]: {
          ...current,
          ...patch,
        },
      };
    });
  }

  function submitWalletRequest() {
    if (!user) {
      return;
    }
    setError(null);
    startTransition(() => {
      void (async () => {
        try {
          await createWalletPresentationRequest(
            {
              ...requestForm,
              credentialType: requestForm.credentialType.trim(),
              purpose: requestForm.purpose?.trim(),
              allowedIssuerDids: allowedIssuerInput
                .split(",")
                .map((value) => value.trim())
                .filter(Boolean),
            },
            user,
          );
          setRequestForm(DEFAULT_REQUEST_FORM);
          setAllowedIssuerInput("");
          await refresh();
        } catch (submitError) {
          console.error("Failed to create wallet presentation request", submitError);
          setError("建立 wallet verification request 失敗。");
        }
      })();
    });
  }

  function submitWalletPresentation(request: WalletPresentationRequest) {
    if (!user) {
      return;
    }
    const draft = completionDrafts[request.id];
    if (!draft?.issuerDid || !draft.claimsJson.trim() || !draft.proof.trim()) {
      setError("請先填寫 issuer、claims 與 proof。");
      return;
    }
    setError(null);
    startTransition(() => {
      void (async () => {
        try {
          await completeWalletPresentationRequest(
            request.id,
            {
              issuerDid: draft.issuerDid,
              credentialType: request.credentialType,
              presentationFormat: "mock_wallet",
              claimsJson: draft.claimsJson,
              proof: draft.proof,
              audience: request.requestUri,
              nonce: request.challenge,
            },
            user,
          );
          await refresh();
        } catch (submitError) {
          console.error("Failed to complete wallet presentation request", submitError);
          setError("送出 wallet presentation 失敗。");
        }
      })();
    });
  }

  return (
    <section className="mt-4 rounded-2xl border border-[#ece7f4] bg-white p-4">
      <div className="flex items-center justify-between gap-4">
        <div>
          <div className="text-[10px] font-bold uppercase tracking-[0.2em] text-[#9a92aa]">Wallet Verification</div>
          <p className="mt-2 text-xs leading-5 text-[#5d5c74]">
            以手機 digital wallet 提交 presentation，Aleth 會檢查 nonce、audience、issuer registry 與 trust tier policy。
          </p>
        </div>
        <div className="rounded-full bg-[#f6f1ff] px-3 py-1 text-[10px] font-bold uppercase tracking-[0.18em] text-[#6424d8]">
          Current L{sessionTier}
        </div>
      </div>

      {error ? (
        <div className="mt-4 rounded-2xl border border-[#f0bfd0] bg-[#fff3f7] px-4 py-3 text-xs text-[#8f2e51]">{error}</div>
      ) : null}

      <div className="mt-4 grid gap-4 xl:grid-cols-[320px_1fr]">
        <div className="rounded-[24px] border border-[#ece7f4] bg-[#faf7ff] p-4">
          <div className="text-[10px] font-bold uppercase tracking-[0.2em] text-[#9a92aa]">Start Request</div>
          <div className="mt-4 space-y-3">
            <select
              value={requestForm.requestedTier}
              onChange={(event) =>
                setRequestForm((prev) => ({ ...prev, requestedTier: Number(event.target.value) }))
              }
              className="w-full rounded-xl border border-[#ece7f4] bg-white px-3 py-2 text-sm text-[#1a1c1c] outline-none"
            >
              <option value={2}>Request L2</option>
              <option value={3}>Request L3</option>
            </select>
            <input
              value={requestForm.credentialType}
              onChange={(event) => setRequestForm((prev) => ({ ...prev, credentialType: event.target.value }))}
              className="w-full rounded-xl border border-[#ece7f4] bg-white px-3 py-2 text-sm text-[#1a1c1c] outline-none"
              placeholder="organization_membership"
            />
            <textarea
              value={requestForm.purpose ?? ""}
              onChange={(event) => setRequestForm((prev) => ({ ...prev, purpose: event.target.value }))}
              className="min-h-24 w-full rounded-xl border border-[#ece7f4] bg-white px-3 py-2 text-sm leading-6 text-[#1a1c1c] outline-none"
              placeholder="Why this wallet verification is needed"
            />
            <input
              value={allowedIssuerInput}
              onChange={(event) => setAllowedIssuerInput(event.target.value)}
              className="w-full rounded-xl border border-[#ece7f4] bg-white px-3 py-2 text-sm text-[#1a1c1c] outline-none"
              placeholder="Allowed issuers, comma separated"
            />
            <select
              value={requestForm.expiresInMinutes}
              onChange={(event) =>
                setRequestForm((prev) => ({ ...prev, expiresInMinutes: Number(event.target.value) }))
              }
              className="w-full rounded-xl border border-[#ece7f4] bg-white px-3 py-2 text-sm text-[#1a1c1c] outline-none"
            >
              <option value={10}>Expires in 10 min</option>
              <option value={15}>Expires in 15 min</option>
              <option value={30}>Expires in 30 min</option>
            </select>
            <button
              type="button"
              onClick={submitWalletRequest}
              disabled={isPending}
              className="w-full rounded-xl bg-[#6424d8] px-4 py-2.5 text-sm font-bold text-white transition hover:bg-[#5210aa] disabled:opacity-50"
            >
              {isPending ? "Submitting..." : "Create Wallet Request"}
            </button>
          </div>
        </div>

        <div className="space-y-4">
          {requests.length === 0 ? (
            <div className="rounded-[24px] border border-dashed border-[#d7d0e5] bg-[#fcfbff] p-6 text-sm text-[#7b7487]">
              尚未建立 wallet verification request。
            </div>
          ) : (
            requests.map((request) => {
              const eligibleIssuers = issuers.filter((issuer) => {
                const allowedByRequest =
                  request.allowedIssuerDids.length === 0 || request.allowedIssuerDids.includes(issuer.issuerDid);
                return (
                  issuer.status === "active" &&
                  allowedByRequest &&
                  issuer.maxTrustTierIssued >= request.requestedTier &&
                  issuer.credentialTypes.includes(request.credentialType)
                );
              });
              const draft = completionDrafts[request.id] ?? {
                issuerDid: eligibleIssuers[0]?.issuerDid ?? "",
                claimsJson: "",
                proof: "wallet-signature-placeholder",
              };
              return (
                <article key={request.id} className="rounded-[24px] border border-[#ece7f4] bg-[#fcfbff] p-5 shadow-sm">
                  <div className="flex flex-col justify-between gap-3 lg:flex-row">
                    <div>
                      <div className="text-[10px] font-bold uppercase tracking-[0.18em] text-[#9a92aa]">{request.id}</div>
                      <h3 className="mt-2 text-lg font-extrabold tracking-tight text-[#1a1c1c]">
                        L{request.requestedTier} · {request.credentialType}
                      </h3>
                      <p className="mt-2 text-xs leading-5 text-[#5d5c74]">
                        {request.purpose || "Wallet-backed verification request"}
                      </p>
                    </div>
                    <div className="rounded-full bg-white px-3 py-1 text-[10px] font-bold uppercase tracking-[0.16em] text-[#6424d8]">
                      {request.status}
                    </div>
                  </div>

                  <div className="mt-4 grid gap-3 md:grid-cols-2">
                    <div className="rounded-2xl bg-white p-3">
                      <div className="text-[10px] font-bold uppercase tracking-[0.16em] text-[#9a92aa]">Request URI</div>
                      <div className="mt-2 break-all font-mono text-[11px] text-[#4d4560]">{request.requestUri}</div>
                    </div>
                    <div className="rounded-2xl bg-white p-3">
                      <div className="text-[10px] font-bold uppercase tracking-[0.16em] text-[#9a92aa]">Challenge</div>
                      <div className="mt-2 break-all font-mono text-[11px] text-[#4d4560]">{request.challenge}</div>
                    </div>
                  </div>

                  <div className="mt-3 rounded-2xl bg-white p-3">
                    <div className="text-[10px] font-bold uppercase tracking-[0.16em] text-[#9a92aa]">QR Payload</div>
                    <pre className="mt-2 overflow-auto text-[11px] leading-5 text-[#4d4560]">{request.qrPayload}</pre>
                  </div>

                  <div className="mt-3 text-[11px] text-[#7b7487]">
                    Allowed issuers: {request.allowedIssuerDids.length > 0 ? request.allowedIssuerDids.join(", ") : "any trusted issuer"}
                  </div>

                  {request.verification ? (
                    <div className="mt-4 rounded-2xl border border-[#ece7f4] bg-white p-4">
                      <div className="flex items-center justify-between gap-3">
                        <div className="text-[10px] font-bold uppercase tracking-[0.18em] text-[#9a92aa]">Verification Result</div>
                        <div className={request.verification.status === "verified" ? "rounded-full bg-[#edf8ef] px-3 py-1 text-[10px] font-bold uppercase tracking-[0.16em] text-[#2f8b57]" : "rounded-full bg-[#fff1f4] px-3 py-1 text-[10px] font-bold uppercase tracking-[0.16em] text-[#b43563]"}>
                          {request.verification.status}
                        </div>
                      </div>
                      <p className="mt-3 text-xs leading-5 text-[#5d5c74]">{request.verification.notes || "No additional notes."}</p>
                      <div className="mt-3 grid gap-2 text-[11px] text-[#7b7487] md:grid-cols-2">
                        <div>Issuer: {request.verification.issuerDid}</div>
                        <div>Format: {request.verification.presentationFormat}</div>
                      </div>
                    </div>
                  ) : request.status === "pending" ? (
                    <div className="mt-4 rounded-2xl border border-[#ece7f4] bg-white p-4">
                      <div className="text-[10px] font-bold uppercase tracking-[0.18em] text-[#9a92aa]">Prototype Completion</div>
                      <p className="mt-2 text-xs leading-5 text-[#5d5c74]">
                        這裡模擬手機 wallet 回傳 presentation。正式版會由 OpenID4VP / Digital Credentials API 送進同一支 API。
                      </p>
                      <div className="mt-4 space-y-3">
                        <select
                          value={draft.issuerDid}
                          onChange={(event) => updateCompletionDraft(request.id, { issuerDid: event.target.value })}
                          className="w-full rounded-xl border border-[#ece7f4] bg-[#fafafa] px-3 py-2 text-sm text-[#1a1c1c] outline-none"
                        >
                          <option value="">Select issuer</option>
                          {eligibleIssuers.map((issuer) => (
                            <option key={issuer.issuerDid} value={issuer.issuerDid}>
                              {issuer.issuerName} · {issuer.issuerDid}
                            </option>
                          ))}
                        </select>
                        <textarea
                          value={draft.claimsJson}
                          onChange={(event) => updateCompletionDraft(request.id, { claimsJson: event.target.value })}
                          className="min-h-24 w-full rounded-xl border border-[#ece7f4] bg-[#fafafa] px-3 py-2 text-sm leading-6 text-[#1a1c1c] outline-none"
                        />
                        <input
                          value={draft.proof}
                          onChange={(event) => updateCompletionDraft(request.id, { proof: event.target.value })}
                          className="w-full rounded-xl border border-[#ece7f4] bg-[#fafafa] px-3 py-2 text-sm text-[#1a1c1c] outline-none"
                          placeholder="signed presentation proof"
                        />
                        <button
                          type="button"
                          onClick={() => submitWalletPresentation(request)}
                          disabled={isPending}
                          className="rounded-xl bg-[#1a1c1c] px-4 py-2.5 text-sm font-bold text-white transition hover:bg-[#000000] disabled:opacity-50"
                        >
                          {isPending ? "Verifying..." : "Complete With Mock Wallet"}
                        </button>
                      </div>
                    </div>
                  ) : null}
                </article>
              );
            })
          )}
        </div>
      </div>
    </section>
  );
}
