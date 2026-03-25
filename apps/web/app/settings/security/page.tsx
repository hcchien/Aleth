"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useAuth } from "@/lib/auth-context";
import { gqlClient } from "@/lib/gql-client";

const REGISTER_PASSKEY_MUTATION = `
  mutation RegisterPasskey($credentialID: String!, $credentialPublicKey: String!, $signCount: Int!) {
    registerPasskey(
      credentialID: $credentialID
      credentialPublicKey: $credentialPublicKey
      signCount: $signCount
    ) {
      accessToken
      refreshToken
      user { id username displayName email emailVerified trustLevel apEnabled }
    }
  }
`;

const MY_CREDENTIAL_TYPES_QUERY = `
  query MyCredentialTypes {
    myCredentialTypes
  }
`;

const DISCONNECT_OAUTH_MUTATION = `
  mutation DisconnectOAuth($provider: String!) {
    disconnectOAuth(provider: $provider)
  }
`;

const DELETE_ACCOUNT_MUTATION = `
  mutation DeleteAccount {
    deleteAccount
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
    emailVerified: boolean;
    apEnabled: boolean;
  };
}

const PROVIDER_LABELS: Record<string, string> = {
  passkey: "Passkey",
  google: "Google",
  facebook: "Facebook",
  password: "Email & Password",
};

function toBase64Url(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (const b of bytes) binary += String.fromCharCode(b);
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}

function randomChallenge(size = 32): ArrayBuffer {
  const bytes = new Uint8Array(size);
  crypto.getRandomValues(bytes);
  return bytes.buffer;
}

function utf8ToArrayBuffer(input: string): ArrayBuffer {
  const encoded = new TextEncoder().encode(input);
  const out = new Uint8Array(encoded.length);
  out.set(encoded);
  return out.buffer;
}

export default function SecuritySettingsPage() {
  const t = useTranslations("security");
  const { user, login, logout } = useAuth();
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [credTypes, setCredTypes] = useState<string[] | null>(null);
  const [disconnecting, setDisconnecting] = useState<string | null>(null);
  const [justRegisteredPasskey, setJustRegisteredPasskey] = useState(false);
  const [deleteConfirm, setDeleteConfirm] = useState("");
  const [deleting, setDeleting] = useState(false);
  const displayName = useMemo(() => user?.displayName ?? user?.username ?? "user", [user]);

  useEffect(() => {
    if (!user) return;
    gqlClient<{ myCredentialTypes: string[] }>(MY_CREDENTIAL_TYPES_QUERY)
      .then((d) => setCredTypes(d.myCredentialTypes))
      .catch(() => setCredTypes([]));
  }, [user]);

  const hasPasskey = credTypes?.includes("passkey") ?? false;
  const oauthProviders = (credTypes ?? []).filter((t) => t === "google" || t === "facebook");

  async function handleRegisterPasskey() {
    setError(null);
    setMessage(null);
    if (!user) {
      setError(t("pleaseSignIn"));
      return;
    }
    if (typeof window === "undefined" || !window.PublicKeyCredential) {
      setError(t("passkeyNotSupported"));
      return;
    }

    setLoading(true);
    try {
      const credential = (await navigator.credentials.create({
        publicKey: {
          challenge: randomChallenge(),
          rp: { name: "Aleth" },
          user: {
            id: utf8ToArrayBuffer(user.id),
            name: user.username,
            displayName,
          },
          pubKeyCredParams: [
            { type: "public-key", alg: -7 },
            { type: "public-key", alg: -257 },
          ],
          timeout: 60_000,
          authenticatorSelection: {
            residentKey: "preferred",
            userVerification: "preferred",
          },
          attestation: "none",
        },
      })) as PublicKeyCredential | null;

      if (!credential) {
        throw new Error(t("passkeyCancelled"));
      }
      const response = credential.response as AuthenticatorAttestationResponse;
      const credentialID = toBase64Url(credential.rawId);
      const credentialPublicKey = toBase64Url(response.attestationObject);

      const data = await gqlClient<{ registerPasskey: AuthPayload }>(
        REGISTER_PASSKEY_MUTATION,
        {
          credentialID,
          credentialPublicKey,
          signCount: 0,
        }
      );

      login(
        data.registerPasskey.accessToken,
        data.registerPasskey.refreshToken,
        data.registerPasskey.user
      );
      setMessage(t("passkeySuccess"));
      setCredTypes((prev) =>
        prev?.includes("passkey") ? prev : [...(prev ?? []), "passkey"]
      );
      // Show the "you can now disconnect your social login" hint if applicable
      if (oauthProviders.length > 0) {
        setJustRegisteredPasskey(true);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t("passkeyFailed"));
    } finally {
      setLoading(false);
    }
  }

  async function handleDisconnect(provider: string) {
    const label = PROVIDER_LABELS[provider] ?? provider;
    if (!confirm(t("disconnectConfirm", { provider: label }))) return;

    setDisconnecting(provider);
    setError(null);
    try {
      await gqlClient<{ disconnectOAuth: boolean }>(DISCONNECT_OAUTH_MUTATION, { provider });
      setCredTypes((prev) => (prev ?? []).filter((t) => t !== provider));
      setMessage(t("disconnectSuccess", { provider: label }));
      setJustRegisteredPasskey(false);
    } catch {
      setError(t("disconnectFailed"));
    } finally {
      setDisconnecting(null);
    }
  }

  async function handleDeleteAccount() {
    if (deleteConfirm !== "DELETE") return;
    setDeleting(true);
    setError(null);
    try {
      await gqlClient<{ deleteAccount: boolean }>(DELETE_ACCOUNT_MUTATION);
      logout();
    } catch {
      setError(t("deleteAccountFailed"));
      setDeleting(false);
    }
  }

  return (
    <div className="mx-auto mt-10 max-w-2xl px-4">
      <Link href="/" className="text-sm text-[var(--app-text-muted)] hover:text-[var(--app-text)]">
        {t("backHome")}
      </Link>
      <h1 className="mt-4 text-2xl font-semibold text-[var(--app-text-heading)]">{t("title")}</h1>
      <p className="mt-2 text-sm text-[var(--app-text-muted)]">
        {t("description")}
      </p>

      {/* Passkey setup */}
      <div className="mt-6 rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)] p-5">
        <p className="text-sm text-[var(--app-text-secondary)]">
          {t("currentAccount")} <span className="font-medium text-[var(--app-text)]">{displayName}</span>
        </p>
        <p className="mt-1 text-sm text-[var(--app-text-secondary)]">
          {t("currentLevel")} <span className="font-medium text-[var(--app-text)]">L{user?.trustLevel ?? 0}</span>
        </p>

        <button
          type="button"
          disabled={loading || !user}
          onClick={handleRegisterPasskey}
          className="mt-4 rounded-lg bg-[var(--app-accent)] px-4 py-2 text-sm font-medium text-white hover:opacity-90 transition-opacity disabled:opacity-50"
        >
          {loading ? t("settingUp") : t("setupPasskey")}
        </button>

        {message && <p className="mt-3 text-sm text-green-600 dark:text-green-400">{message}</p>}
        {error && <p className="mt-3 text-sm text-red-500">{error}</p>}
      </div>

      {/* Post-passkey hint */}
      {justRegisteredPasskey && oauthProviders.length > 0 && (
        <div className="mt-4 rounded-xl border border-[var(--app-accent-border)] bg-[var(--app-surface-2)] px-5 py-4 text-sm text-[var(--app-text-secondary)]">
          {t("passkeyReadyHint")}
        </div>
      )}

      {/* Connected login methods */}
      {credTypes !== null && credTypes.length > 0 && (
        <div className="mt-8">
          <h2 className="mb-3 text-base font-semibold text-[var(--app-text-heading)]">
            {t("connectedAccounts")}
          </h2>
          <div className="divide-y divide-[var(--app-border)] rounded-xl border border-[var(--app-border-2)] bg-[var(--app-surface)]">
            {credTypes.map((credType) => (
              <div key={credType} className="flex items-center justify-between px-5 py-3">
                <span className="text-sm font-medium text-[var(--app-text)]">
                  {PROVIDER_LABELS[credType] ?? credType}
                </span>
                {(credType === "google" || credType === "facebook") && (
                  <button
                    type="button"
                    disabled={disconnecting === credType}
                    onClick={() => void handleDisconnect(credType)}
                    className="text-xs text-[var(--app-text-muted)] hover:text-red-500 transition-colors disabled:opacity-50"
                  >
                    {disconnecting === credType ? t("disconnecting") : t("disconnectOAuth")}
                  </button>
                )}
              </div>
            ))}
          </div>
          <p className="mt-2 text-xs text-[var(--app-text-dim)]">
            {hasPasskey && oauthProviders.length > 0
              ? t("passkeyReadyHint")
              : null}
          </p>
        </div>
      )}

      {/* Danger zone */}
      <div className="mt-12">
        <h2 className="mb-3 text-base font-semibold text-red-600 dark:text-red-400">
          {t("dangerZone")}
        </h2>
        <div className="rounded-xl border border-red-200 bg-red-50 p-5 dark:border-red-900 dark:bg-red-950/30">
          <p className="text-sm font-medium text-[var(--app-text)]">{t("deleteAccount")}</p>
          <p className="mt-1 text-sm text-[var(--app-text-secondary)]">{t("deleteAccountDesc")}</p>
          <div className="mt-4 space-y-2">
            <label className="block text-xs font-medium text-[var(--app-text-muted)]">
              {t("deleteAccountConfirm")}
            </label>
            <input
              type="text"
              value={deleteConfirm}
              onChange={(e) => setDeleteConfirm(e.target.value)}
              placeholder={t("deleteAccountConfirmPlaceholder")}
              className="w-full rounded-lg border border-[var(--app-border-2)] bg-[var(--app-bg)] px-3 py-2 text-sm font-mono text-[var(--app-text)] focus:outline-none focus:ring-2 focus:ring-red-400"
            />
            <button
              type="button"
              disabled={deleteConfirm !== "DELETE" || deleting || !user}
              onClick={() => void handleDeleteAccount()}
              className="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {deleting ? t("deleting") : t("deleteAccountButton")}
            </button>
          </div>
          {error && <p className="mt-3 text-sm text-red-500">{error}</p>}
        </div>
      </div>
    </div>
  );
}
