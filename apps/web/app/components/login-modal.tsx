"use client";

import { useState, FormEvent, useEffect } from "react";
import Script from "next/script";
import Link from "next/link";
import { gqlClient } from "@/lib/gql-client";
import { useAuth } from "@/lib/auth-context";

const LOGIN_MUTATION = `
  mutation Login($input: LoginInput!) {
    login(input: $input) {
      accessToken
      refreshToken
      user { id username displayName email trustLevel apEnabled }
    }
  }
`;

const LOGIN_WITH_GOOGLE_MUTATION = `
  mutation LoginWithGoogle($idToken: String!) {
    loginWithGoogle(idToken: $idToken) {
      accessToken
      refreshToken
      user { id username displayName email trustLevel apEnabled }
    }
  }
`;

const BEGIN_PASSKEY_LOGIN_MUTATION = `
  mutation BeginPasskeyLogin($username: String) {
    beginPasskeyLogin(username: $username) {
      challenge
      challengeToken
      rpId
      timeoutMs
      allowCredentialIds
    }
  }
`;

const FINISH_PASSKEY_LOGIN_MUTATION = `
  mutation FinishPasskeyLogin($input: PasskeyAssertionInput!) {
    finishPasskeyLogin(input: $input) {
      accessToken
      refreshToken
      user { id username displayName email trustLevel apEnabled }
    }
  }
`;

interface AuthPayload {
  accessToken: string;
  refreshToken: string;
  user: {
    id: string;
    username: string;
    displayName: string | null;
    email: string | null;
    trustLevel: number;
    apEnabled: boolean;
  };
}

interface GoogleIDClient {
  initialize(config: {
    client_id: string;
    callback: (resp: { credential?: string }) => void;
    ux_mode?: "popup" | "redirect";
  }): void;
  prompt(): void;
}

function decodeBase64URL(value: string): ArrayBuffer {
  const pad = value.length % 4 === 0 ? "" : "=".repeat(4 - (value.length % 4));
  const base64 = (value + pad).replace(/-/g, "+").replace(/_/g, "/");
  const binary = atob(base64);
  const out = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) out[i] = binary.charCodeAt(i);
  return out.buffer;
}

function encodeBase64URL(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (const b of bytes) binary += String.fromCharCode(b);
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}

const TRUST_TABLE = [
  { level: "L0", name: "讀者", desc: "Google 登入，可瀏覽內容", className: "text-gray-400" },
  { level: "L1", name: "基本成員", desc: "Passkey 驗證，可發文互動", className: "text-teal-300" },
  { level: "L2", name: "進階成員", desc: "活躍貢獻者", className: "text-lime-300" },
  { level: "L3", name: "可信成員", desc: "完成數位皮夾憑證驗證", className: "text-amber-300" },
  { level: "L4", name: "管理者", desc: "管理員授予", className: "text-purple-300" },
];

export function LoginModal({
  isOpen,
  onClose,
}: {
  isOpen: boolean;
  onClose: () => void;
}) {
  const { login } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [passkeyUsername, setPasskeyUsername] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState<
    "email" | "google" | "passkey" | null
  >(null);
  const [showEmailForm, setShowEmailForm] = useState(false);

  const googleClientID = process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID ?? "";

  // Close on Escape
  useEffect(() => {
    if (!isOpen) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  async function handleFinish(payload: AuthPayload) {
    login(payload.accessToken, payload.refreshToken, payload.user);
    onClose();
  }

  async function handleEmailSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading("email");
    try {
      const data = await gqlClient<{ login: AuthPayload }>(LOGIN_MUTATION, {
        input: { email, password },
      });
      await handleFinish(data.login);
    } catch (err) {
      setError(err instanceof Error ? err.message : "登入失敗");
    } finally {
      setLoading(null);
    }
  }

  async function handleGoogleLogin() {
    setError(null);
    if (!googleClientID) {
      setError("Google OAuth 未設定 (NEXT_PUBLIC_GOOGLE_CLIENT_ID)");
      return;
    }
    const g = (
      window as Window & {
        google?: { accounts?: { id?: GoogleIDClient } };
      }
    ).google;
    const idClient = g?.accounts?.id;
    if (!idClient) {
      setError("Google SDK 尚未載入，請稍後再試");
      return;
    }
    setLoading("google");
    idClient.initialize({
      client_id: googleClientID,
      callback: async (resp: { credential?: string }) => {
        try {
          if (!resp?.credential) throw new Error("Google 驗證失敗");
          const data = await gqlClient<{ loginWithGoogle: AuthPayload }>(
            LOGIN_WITH_GOOGLE_MUTATION,
            { idToken: resp.credential }
          );
          await handleFinish(data.loginWithGoogle);
        } catch (err) {
          setError(err instanceof Error ? err.message : "Google 登入失敗");
        } finally {
          setLoading(null);
        }
      },
      ux_mode: "popup",
    });
    idClient.prompt();
  }

  async function handlePasskeyLogin() {
    setError(null);
    if (typeof window === "undefined" || !window.PublicKeyCredential) {
      setError("此瀏覽器不支援 Passkey");
      return;
    }
    setLoading("passkey");
    try {
      const username = passkeyUsername.trim() || undefined;
      const begin = await gqlClient<{
        beginPasskeyLogin: {
          challenge: string;
          challengeToken: string;
          rpId: string;
          timeoutMs: number;
          allowCredentialIds: string[];
        };
      }>(BEGIN_PASSKEY_LOGIN_MUTATION, { username });
      const options = begin.beginPasskeyLogin;

      const assertion = (await navigator.credentials.get({
        publicKey: {
          challenge: decodeBase64URL(options.challenge),
          rpId: options.rpId,
          timeout: options.timeoutMs,
          userVerification: "preferred",
          ...(options.allowCredentialIds.length > 0
            ? {
                allowCredentials: options.allowCredentialIds.map((id) => ({
                  type: "public-key" as PublicKeyCredentialType,
                  id: decodeBase64URL(id),
                })),
              }
            : {}),
        },
      })) as PublicKeyCredential | null;
      if (!assertion) throw new Error("Passkey 登入已取消");
      const response = assertion.response as AuthenticatorAssertionResponse;

      const finish = await gqlClient<{ finishPasskeyLogin: AuthPayload }>(
        FINISH_PASSKEY_LOGIN_MUTATION,
        {
          input: {
            credentialID: encodeBase64URL(assertion.rawId),
            challengeToken: options.challengeToken,
            clientDataJSON: encodeBase64URL(response.clientDataJSON),
            authenticatorData: encodeBase64URL(response.authenticatorData),
            signature: encodeBase64URL(response.signature),
            userHandle: response.userHandle
              ? encodeBase64URL(response.userHandle)
              : null,
            username: username ?? null,
          },
        }
      );
      await handleFinish(finish.finishPasskeyLogin);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Passkey 登入失敗");
    } finally {
      setLoading(null);
    }
  }

  return (
    <>
      {googleClientID && (
        <Script
          src="https://accounts.google.com/gsi/client"
          strategy="afterInteractive"
        />
      )}
      {/* Backdrop */}
      <div
        className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
        onClick={(e) => {
          if (e.target === e.currentTarget) onClose();
        }}
      >
        <div className="relative w-full max-w-md rounded-2xl border border-[#2a2e38] bg-[#161923] p-8 shadow-2xl">
          {/* Close */}
          <button
            type="button"
            onClick={onClose}
            className="absolute right-4 top-4 text-lg text-[#7a8090] hover:text-white"
            aria-label="關閉"
          >
            ✕
          </button>

          {/* Header */}
          <h2 className="mb-6 font-serif text-2xl text-[#f3f5f9]">
            登入 <span className="text-[#f09a45]">Aleth</span>
          </h2>

          {/* Main auth buttons */}
          <div className="flex flex-col gap-3 mb-6">
            <button
              type="button"
              onClick={handleGoogleLogin}
              disabled={loading !== null}
              className="flex items-center gap-3 rounded-xl border border-[#353b48] bg-[#1e232e] px-5 py-4 text-left hover:border-[#4a5060] hover:bg-[#232a38] disabled:opacity-50"
            >
              <span className="flex h-9 w-9 items-center justify-center rounded-full bg-white text-base font-bold text-[#4285F4]">
                G
              </span>
              <div className="flex flex-col">
                <span className="text-sm font-medium text-[#e6e7ea]">
                  {loading === "google" ? "登入中…" : "Google 帳號"}
                </span>
                <span className="text-xs text-[#7a8090]">
                  ◎ L0 等級，可瀏覽內容
                </span>
              </div>
            </button>

            <button
              type="button"
              onClick={handlePasskeyLogin}
              disabled={loading !== null}
              className="flex items-center gap-3 rounded-xl border border-[#353b48] bg-[#1e232e] px-5 py-4 text-left hover:border-[#4a5060] hover:bg-[#232a38] disabled:opacity-50"
            >
              <span className="flex h-9 w-9 items-center justify-center rounded-full bg-[#1a2535] text-base text-teal-300">
                🔑
              </span>
              <div className="flex flex-col gap-0.5">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-[#e6e7ea]">
                    {loading === "passkey" ? "驗證中…" : "Passkey 登入"}
                  </span>
                  {loading !== "passkey" && (
                    <input
                      type="text"
                      value={passkeyUsername}
                      onChange={(e) => setPasskeyUsername(e.target.value)}
                      onClick={(e) => e.stopPropagation()}
                      placeholder="使用者名稱（可選）"
                      className="w-32 rounded-md border border-[#353b48] bg-[#0f1117] px-2 py-0.5 text-xs text-[#d0d4de] placeholder:text-[#5a6070] focus:outline-none focus:border-teal-500"
                    />
                  )}
                </div>
                <span className="text-xs text-[#7a8090]">
                  ◈ L1 或以上，可發文互動
                </span>
              </div>
            </button>
          </div>

          {/* Error */}
          {error && (
            <p className="mb-4 rounded-lg border border-red-900/50 bg-red-950/30 px-4 py-2 text-sm text-red-400">
              {error}
            </p>
          )}

          {/* Email/password toggle */}
          <div className="mb-6">
            <button
              type="button"
              onClick={() => setShowEmailForm((v) => !v)}
              className="text-xs text-[#6a7080] hover:text-[#aeb4bf] underline"
            >
              {showEmailForm ? "▲ 收起電子郵件登入" : "▼ 電子郵件登入"}
            </button>
            {showEmailForm && (
              <form
                onSubmit={handleEmailSubmit}
                className="mt-3 flex flex-col gap-3"
              >
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                  placeholder="電子郵件"
                  className="w-full rounded-lg border border-[#2a2e38] bg-[#0f1117] px-4 py-2.5 text-sm text-[#e6e7ea] placeholder:text-[#5a6070] focus:border-[#4a5060] focus:outline-none"
                />
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  placeholder="密碼"
                  className="w-full rounded-lg border border-[#2a2e38] bg-[#0f1117] px-4 py-2.5 text-sm text-[#e6e7ea] placeholder:text-[#5a6070] focus:border-[#4a5060] focus:outline-none"
                />
                <button
                  type="submit"
                  disabled={loading !== null}
                  className="rounded-lg bg-[#2a3448] px-4 py-2.5 text-sm font-medium text-[#e6e7ea] hover:bg-[#333d52] disabled:opacity-50"
                >
                  {loading === "email" ? "登入中…" : "登入"}
                </button>
              </form>
            )}
          </div>

          {/* Trust level table */}
          <div className="rounded-xl border border-[#252932] bg-[#0f1117] px-4 py-3">
            <p className="mb-2 text-xs font-medium text-[#8a909e]">
              信任等級說明
            </p>
            <table className="w-full text-xs">
              <tbody>
                {TRUST_TABLE.map((row) => (
                  <tr key={row.level} className="border-t border-[#1e2330] first:border-t-0">
                    <td className={`py-1.5 pr-3 font-mono font-medium ${row.className}`}>
                      {row.level}
                    </td>
                    <td className="py-1.5 pr-3 text-[#c8cdd8]">{row.name}</td>
                    <td className="py-1.5 text-[#7a8090]">{row.desc}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <p className="mt-4 text-center text-xs text-[#5a6070]">
            沒有帳號？{" "}
            <Link
              href="/register"
              onClick={onClose}
              className="text-[#7ea3ff] hover:underline"
            >
              註冊
            </Link>
          </p>
        </div>
      </div>
    </>
  );
}
