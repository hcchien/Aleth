"use client";

import { useState, FormEvent } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import Script from "next/script";
import { useTranslations } from "next-intl";
import { gqlClient } from "@/lib/gql-client";
import { useAuth } from "@/lib/auth-context";

const LOGIN_MUTATION = `
  mutation Login($input: LoginInput!) {
    login(input: $input) {
      accessToken
      refreshToken
      user { id username displayName email trustLevel }
    }
  }
`;

const LOGIN_WITH_GOOGLE_MUTATION = `
  mutation LoginWithGoogle($idToken: String!) {
    loginWithGoogle(idToken: $idToken) {
      accessToken
      refreshToken
      user { id username displayName email trustLevel }
    }
  }
`;

const LOGIN_WITH_FACEBOOK_MUTATION = `
  mutation LoginWithFacebook($accessToken: String!) {
    loginWithFacebook(accessToken: $accessToken) {
      accessToken
      refreshToken
      user { id username displayName email trustLevel }
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

interface GoogleIDClient {
  initialize(config: {
    client_id: string;
    callback: (resp: { credential?: string }) => void;
    ux_mode?: "popup" | "redirect";
  }): void;
  prompt(): void;
}

interface FacebookSDK {
  init(config: {
    appId: string;
    cookie: boolean;
    xfbml: boolean;
    version: string;
  }): void;
  login(
    callback: (resp: { authResponse?: { accessToken?: string } }) => void,
    options: { scope: string }
  ): void;
}

export default function LoginPage() {
  const t = useTranslations("login");
  const router = useRouter();
  const searchParams = useSearchParams();
  const redirectTo = searchParams.get("redirect") ?? "/";
  const { login } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [oauthLoading, setOauthLoading] = useState<"google" | "facebook" | "passkey" | null>(null);
  const [passkeyUsername, setPasskeyUsername] = useState("");
  const googleClientID = process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID ?? "";
  const facebookAppID = process.env.NEXT_PUBLIC_FACEBOOK_APP_ID ?? "";

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      const data = await gqlClient<{ login: AuthPayload }>(LOGIN_MUTATION, {
        input: { email, password },
      });
      login(data.login.accessToken, data.login.refreshToken, data.login.user);
      router.push(redirectTo);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("errors.loginFailed"));
    } finally {
      setLoading(false);
    }
  }

  async function finishOAuthLogin(payload: AuthPayload) {
    login(payload.accessToken, payload.refreshToken, payload.user);
    router.push(redirectTo);
  }

  async function handleGoogleLogin() {
    setError(null);
    if (!googleClientID) {
      setError(t("errors.googleNotConfigured"));
      return;
    }
    const g = (window as Window & {
      google?: { accounts?: { id?: GoogleIDClient } };
    }).google;
    const idClient = g?.accounts?.id;
    if (!idClient) {
      setError(t("errors.googleSdkNotLoaded"));
      return;
    }
    setOauthLoading("google");
    idClient.initialize({
      client_id: googleClientID,
      callback: async (resp: { credential?: string }) => {
        try {
          if (!resp?.credential) throw new Error(t("errors.googleMissingCredential"));
          const data = await gqlClient<{ loginWithGoogle: AuthPayload }>(
            LOGIN_WITH_GOOGLE_MUTATION,
            { idToken: resp.credential }
          );
          await finishOAuthLogin(data.loginWithGoogle);
        } catch (err) {
          setError(err instanceof Error ? err.message : t("errors.googleFailed"));
        } finally {
          setOauthLoading(null);
        }
      },
      ux_mode: "popup",
    });
    idClient.prompt();
  }

  async function handleFacebookLogin() {
    setError(null);
    if (!facebookAppID) {
      setError(t("errors.facebookNotConfigured"));
      return;
    }
    const fb = (window as Window & { FB?: FacebookSDK }).FB;
    if (!fb?.login) {
      setError(t("errors.facebookSdkNotLoaded"));
      return;
    }
    setOauthLoading("facebook");
    fb.login(
      async (resp: { authResponse?: { accessToken?: string } }) => {
        try {
          const accessToken = resp?.authResponse?.accessToken;
          if (!accessToken) throw new Error(t("errors.facebookCancelled"));
          const data = await gqlClient<{ loginWithFacebook: AuthPayload }>(
            LOGIN_WITH_FACEBOOK_MUTATION,
            { accessToken }
          );
          await finishOAuthLogin(data.loginWithFacebook);
        } catch (err) {
          setError(err instanceof Error ? err.message : t("errors.facebookFailed"));
        } finally {
          setOauthLoading(null);
        }
      },
      { scope: "email,public_profile" }
    );
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

  async function handlePasskeyLogin() {
    setError(null);
    if (typeof window === "undefined" || !window.PublicKeyCredential) {
      setError(t("errors.passkeyNotSupported"));
      return;
    }
    setOauthLoading("passkey");
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
      if (!assertion) {
        throw new Error(t("errors.passkeyCancelled"));
      }
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
      await finishOAuthLogin(finish.finishPasskeyLogin);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("errors.passkeyFailed"));
    } finally {
      setOauthLoading(null);
    }
  }

  return (
    <div className="max-w-sm mx-auto mt-12">
      <Script src="https://accounts.google.com/gsi/client" strategy="afterInteractive" />
      <Script
        src="https://connect.facebook.net/en_US/sdk.js"
        strategy="afterInteractive"
        onLoad={() => {
          const fb = (window as Window & { FB?: FacebookSDK }).FB;
          if (!fb || !facebookAppID) return;
          fb.init({
            appId: facebookAppID,
            cookie: true,
            xfbml: false,
            version: "v23.0",
          });
        }}
      />
      <h1 className="text-2xl font-semibold mb-6">{t("title")}</h1>
      <div className="flex flex-col gap-2 mb-4">
        <input
          type="text"
          value={passkeyUsername}
          onChange={(e) => setPasskeyUsername(e.target.value)}
          placeholder={t("usernamePlaceholder")}
          className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"
        />
        <button
          type="button"
          onClick={handlePasskeyLogin}
          disabled={oauthLoading !== null}
          className="border border-gray-300 rounded-md px-4 py-2 text-sm font-medium hover:bg-gray-50 disabled:opacity-50"
        >
          {oauthLoading === "passkey" ? t("signingInWithPasskey") : t("continueWithPasskey")}
        </button>
        <button
          type="button"
          onClick={handleGoogleLogin}
          disabled={oauthLoading !== null}
          className="border border-gray-300 rounded-md px-4 py-2 text-sm font-medium hover:bg-gray-50 disabled:opacity-50"
        >
          {oauthLoading === "google" ? t("signingInWithGoogle") : t("continueWithGoogle")}
        </button>
        <button
          type="button"
          onClick={handleFacebookLogin}
          disabled={oauthLoading !== null}
          className="border border-gray-300 rounded-md px-4 py-2 text-sm font-medium hover:bg-gray-50 disabled:opacity-50"
        >
          {oauthLoading === "facebook" ? t("signingInWithFacebook") : t("continueWithFacebook")}
        </button>
      </div>
      <div className="relative my-4">
        <div className="absolute inset-0 flex items-center">
          <span className="w-full border-t border-gray-200" />
        </div>
        <div className="relative flex justify-center text-xs uppercase">
          <span className="bg-white px-2 text-gray-500">{t("or")}</span>
        </div>
      </div>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            {t("email")}
          </label>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            {t("password")}
          </label>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"
          />
        </div>
        {error && <p className="text-red-600 text-sm">{error}</p>}
        <button
          type="submit"
          disabled={loading || oauthLoading !== null}
          className="bg-gray-900 text-white rounded-md px-4 py-2 text-sm font-medium hover:bg-gray-700 disabled:opacity-50"
        >
          {loading ? t("signingIn") : t("signIn")}
        </button>
      </form>
      <p className="mt-4 text-sm text-gray-500">
        {t("noAccount")}{" "}
        <Link href="/register" className="text-gray-900 hover:underline">
          {t("signUp")}
        </Link>
      </p>
    </div>
  );
}
