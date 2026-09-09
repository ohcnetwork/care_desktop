import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

export type Tone = "neutral" | "ok" | "bad";

const TONE_BORDER: Record<Tone, string> = {
  neutral: "border-line",
  ok: "border-brand-line",
  bad: "border-danger-line",
};

/**
 * A bordered field that holds an input plus an adornment (a suffix, a
 * show/hide button, a match note) and carries the validity colour.
 */
export function InputBox({
  tone = "neutral",
  className,
  children,
}: {
  tone?: Tone;
  className?: string;
  children: ReactNode;
}) {
  return (
    <div
      className={cn(
        "flex h-[42px] min-w-0 flex-1 items-center gap-2 rounded-md border bg-white pr-2.5 pl-3 transition-colors",
        TONE_BORDER[tone],
        className,
      )}
    >
      {children}
    </div>
  );
}

/** The muted suffix / status word that sits at the right of an InputBox. */
export function BoxNote({ tone = "neutral", children }: { tone?: Tone; children: ReactNode }) {
  return (
    <span
      className={cn(
        "flex-none text-xs font-semibold",
        tone === "ok" ? "text-brand-ink" : tone === "bad" ? "text-danger-ink" : "text-faint",
      )}
    >
      {children}
    </span>
  );
}
