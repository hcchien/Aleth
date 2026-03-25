"use client";

import { AuthProvider } from "@/lib/auth-context";
import { ReactNode } from "react";
import { EmailVerifyBanner } from "./email-verify-banner";

export function Providers({ children }: { children: ReactNode }) {
  return (
    <AuthProvider>
      <EmailVerifyBanner />
      {children}
    </AuthProvider>
  );
}
