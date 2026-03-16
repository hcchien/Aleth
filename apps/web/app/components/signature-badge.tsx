"use client";

import { useState } from "react";

export interface SignatureInfo {
  isSigned: boolean;
  isVerified: boolean;
  contentHash?: string | null;
  signature?: string | null;
  algorithm?: string | null;
  explanation: string;
}

function short(v: string | null | undefined, head = 10, tail = 8): string {
  if (!v) return "-";
  if (v.length <= head + tail + 3) return v;
  return `${v.slice(0, head)}...${v.slice(-tail)}`;
}

export function SignatureBadge({ info }: { info: SignatureInfo }) {
  const [open, setOpen] = useState(false);
  if (!info.isSigned) return null;

  const color = info.isVerified ? "text-emerald-400" : "text-amber-400";
  const title = info.isVerified ? "已簽章驗證" : "簽章待確認";

  return (
    <span className="group relative inline-flex items-center">
      <button
        type="button"
        aria-label={title}
        title={title}
        onClick={() => setOpen((v) => !v)}
        className={`${color} font-semibold`}
      >
        ✓
      </button>
      <span className="pointer-events-none absolute -top-9 left-1/2 z-20 hidden -translate-x-1/2 rounded-md border border-[#3a3f48] bg-[#151922] px-2 py-1 text-xs whitespace-nowrap text-[#dce1eb] group-hover:block">
        {title}
      </span>
      {open && (
        <div className="absolute right-0 top-6 z-30 w-80 rounded-lg border border-[#3a3f48] bg-[#121722] p-3 text-xs text-[#dce1eb] shadow-xl">
          <p className="mb-2 text-sm font-semibold">{title}</p>
          <p className="mb-2 text-[#aeb6c5]">{info.explanation}</p>
          <div className="space-y-1">
            <p>Hash:</p>
            <p className="rounded bg-[#0c111a] p-2 break-all text-[#e7edf7]">{info.contentHash || "-"}</p>
            <p>Algorithm: <code className="text-[#e7edf7]">{info.algorithm || "sha256"}</code></p>
            <p>Signature: <code className="text-[#e7edf7]">{short(info.signature, 12, 10)}</code></p>
          </div>
          <button
            type="button"
            onClick={() => setOpen(false)}
            className="mt-3 rounded border border-[#3a3f48] px-2 py-1 text-xs hover:bg-[#1a2230]"
          >
            關閉
          </button>
        </div>
      )}
    </span>
  );
}
