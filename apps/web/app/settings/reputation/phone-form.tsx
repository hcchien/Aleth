"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { gqlClient } from "@/lib/gql-client";
import { useAuth } from "@/lib/auth-context";

const REQUEST_OTP = `
  mutation RequestPhoneOTP($phone: String!) {
    requestPhoneOTP(phone: $phone)
  }
`;

const VERIFY_OTP = `
  mutation VerifyPhoneOTP($phone: String!, $code: String!) {
    verifyPhoneOTP(phone: $phone, code: $code) {
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

interface Props {
  /** Called after a successful phone verification so the parent can refresh the stamp list. */
  onVerified: () => void;
  /** Whether the user already has a verified phone stamp. */
  alreadyVerified: boolean;
}

export function PhoneVerificationForm({ onVerified, alreadyVerified }: Props) {
  const t = useTranslations("reputation");
  const { login } = useAuth();

  const [phone, setPhone] = useState("");
  const [code, setCode] = useState("");
  const [step, setStep] = useState<"phone" | "code">("phone");
  const [loading, setLoading] = useState(false);
  const [success, setSuccess] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSendCode(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      await gqlClient<{ requestPhoneOTP: boolean }>(REQUEST_OTP, { phone });
      setStep("code");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to send code");
    } finally {
      setLoading(false);
    }
  }

  async function handleVerifyCode(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      const data = await gqlClient<{ verifyPhoneOTP: AuthPayload }>(VERIFY_OTP, { phone, code });
      // Update the auth context so the trust level refreshes immediately.
      login(
        data.verifyPhoneOTP.accessToken,
        data.verifyPhoneOTP.refreshToken,
        data.verifyPhoneOTP.user
      );
      setSuccess(true);
      onVerified();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Verification failed");
    } finally {
      setLoading(false);
    }
  }

  if (success) {
    return (
      <p className="text-sm font-medium text-lime-400">{t("phoneVerified")}</p>
    );
  }

  return (
    <div className="space-y-3">
      <p className="text-sm font-semibold text-[var(--app-text-bright)]">{t("verifyPhone")}</p>

      {step === "phone" ? (
        <form onSubmit={handleSendCode} className="flex gap-2">
          <input
            type="tel"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
            placeholder={t("phonePlaceholder")}
            required
            className="flex-1 rounded-lg border border-[var(--app-border-2)] bg-[var(--app-input-bg)] px-3 py-2 text-sm text-[var(--app-text)] placeholder:text-[var(--app-text-dim)] focus:border-[var(--app-accent)]/60 focus:outline-none"
          />
          <button
            type="submit"
            disabled={loading || !phone.trim()}
            className="rounded-lg bg-[var(--app-accent)] px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50"
          >
            {loading ? t("sending") : t("sendCode")}
          </button>
        </form>
      ) : (
        <>
          <p className="text-xs text-[var(--app-text-secondary)]">{t("codeSent")}</p>
          <form onSubmit={handleVerifyCode} className="flex gap-2">
            <input
              type="text"
              inputMode="numeric"
              pattern="[0-9]{6}"
              maxLength={6}
              value={code}
              onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))}
              placeholder={t("codePlaceholder")}
              required
              className="w-36 rounded-lg border border-[var(--app-border-2)] bg-[var(--app-input-bg)] px-3 py-2 text-sm text-[var(--app-text)] placeholder:text-[var(--app-text-dim)] tracking-widest focus:border-[var(--app-accent)]/60 focus:outline-none"
            />
            <button
              type="submit"
              disabled={loading || code.length !== 6}
              className="rounded-lg bg-[var(--app-accent)] px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50"
            >
              {loading ? t("verifying") : t("verifyCode")}
            </button>
            <button
              type="button"
              onClick={() => { setStep("phone"); setCode(""); setError(null); }}
              className="text-xs text-[var(--app-text-dim)] hover:text-[var(--app-text-secondary)] underline"
            >
              ‹ back
            </button>
          </form>
        </>
      )}

      {error && <p className="text-xs text-red-400">{error}</p>}
    </div>
  );
}
