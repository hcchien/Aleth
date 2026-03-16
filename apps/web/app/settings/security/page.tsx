"use client";

import { useMemo, useState } from "react";
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
      user { id username displayName email trustLevel }
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
  };
}

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
  const { user, login } = useAuth();
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const displayName = useMemo(() => user?.displayName ?? user?.username ?? "user", [user]);

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
    } catch (err) {
      setError(err instanceof Error ? err.message : t("passkeyFailed"));
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="mx-auto mt-10 max-w-2xl px-4">
      <Link href="/" className="text-sm text-gray-500 hover:text-gray-800">
        {t("backHome")}
      </Link>
      <h1 className="mt-4 text-2xl font-semibold">{t("title")}</h1>
      <p className="mt-2 text-sm text-gray-600">
        {t("description")}
      </p>

      <div className="mt-6 rounded-lg border border-gray-200 p-5">
        <p className="text-sm text-gray-700">
          {t("currentAccount")} <span className="font-medium">{displayName}</span>
        </p>
        <p className="mt-1 text-sm text-gray-700">
          {t("currentLevel")} <span className="font-medium">L{user?.trustLevel ?? 0}</span>
        </p>

        <button
          type="button"
          disabled={loading || !user}
          onClick={handleRegisterPasskey}
          className="mt-4 rounded-md bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-700 disabled:opacity-50"
        >
          {loading ? t("settingUp") : t("setupPasskey")}
        </button>

        {message && <p className="mt-3 text-sm text-green-600">{message}</p>}
        {error && <p className="mt-3 text-sm text-red-600">{error}</p>}
      </div>
    </div>
  );
}
