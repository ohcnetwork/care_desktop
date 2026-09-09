import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

/** The scrolling column to the right of the rail. */
export function Screen({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <div className={cn("flex min-w-0 flex-1 flex-col overflow-hidden", className)}>
      {children}
    </div>
  );
}

export function ScreenHead({
  kicker,
  title,
  subtitle,
  className,
}: {
  kicker?: string;
  title: string;
  subtitle?: string;
  className?: string;
}) {
  return (
    <div className={cn("px-[34px] pt-[30px] pb-5", className)}>
      {kicker ? (
        <div className="mb-1.5 text-xs font-bold tracking-[0.04em] text-brand-ink uppercase">
          {kicker}
        </div>
      ) : null}
      <h1 className="text-2xl font-bold tracking-[-0.015em] text-ink">{title}</h1>
      {subtitle ? <p className="mt-1.5 text-sm text-muted-foreground">{subtitle}</p> : null}
    </div>
  );
}

export function ScreenBody({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <div className={cn("flex-1 overflow-auto px-[34px] pb-[26px]", className)}>{children}</div>
  );
}

export function ScreenFoot({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <div
      className={cn(
        "flex items-center gap-3.5 border-t border-line bg-white px-[34px] py-4",
        className,
      )}
    >
      {children}
    </div>
  );
}

/** The muted sentence that trails a footer's buttons. */
export function FootNote({ children }: { children: ReactNode }) {
  return <span className="text-[13px] text-muted-foreground">{children}</span>;
}
