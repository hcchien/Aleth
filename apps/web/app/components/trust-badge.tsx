export const TRUST_LEVEL_CONFIG: Record<
  number,
  { icon: string; label: string; className: string }
> = {
  0: {
    icon: "◎",
    label: "L0",
    className: "border-gray-500/50 bg-gray-800/40 text-gray-400",
  },
  1: {
    icon: "◈",
    label: "L1",
    className: "border-teal-500/50 bg-teal-900/40 text-teal-300",
  },
  2: {
    icon: "◆",
    label: "L2",
    className: "border-lime-500/50 bg-lime-900/40 text-lime-300",
  },
  3: {
    icon: "⬡",
    label: "L3",
    className: "border-amber-500/50 bg-amber-900/40 text-amber-300",
  },
  4: {
    icon: "✦",
    label: "L4",
    className: "border-purple-500/50 bg-purple-900/40 text-purple-300",
  },
};

export function trustBadgeClass(level: number): string {
  const cfg =
    TRUST_LEVEL_CONFIG[Math.min(4, Math.max(0, level))] ??
    TRUST_LEVEL_CONFIG[0];
  return cfg.className;
}

export function TrustBadge({ level }: { level: number }) {
  const cfg =
    TRUST_LEVEL_CONFIG[Math.min(4, Math.max(0, level))] ??
    TRUST_LEVEL_CONFIG[0];
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-mono ${cfg.className}`}
    >
      <span>{cfg.icon}</span>
      <span>{cfg.label}</span>
    </span>
  );
}
