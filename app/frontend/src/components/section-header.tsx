import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

/** The numbered/ticked circle at the head of a setup section. */
export function StepDot({ children, done }: { children?: ReactNode; done?: boolean }) {
  return (
    <span
      className={cn(
        "flex size-[26px] shrink-0 items-center justify-center rounded-full text-[13px] font-bold",
        done ? "bg-brand text-white" : "bg-hair text-muted-foreground",
      )}
    >
      {done ? "✓" : children}
    </span>
  );
}

/** Title + one-line summary; fills the space between the dot and the badge. */
export function SectionTitle({ title, summary }: { title: ReactNode; summary?: ReactNode }) {
  return (
    <span className="min-w-0 flex-1">
      <span className="block text-[15px] font-semibold text-ink">{title}</span>
      {summary ? (
        <span className="mt-0.5 block text-[13px] font-normal text-muted-foreground">
          {summary}
        </span>
      ) : null}
    </span>
  );
}
