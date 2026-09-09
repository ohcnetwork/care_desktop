import { type ReactNode, useState } from "react";

import { Alert } from "@/components/ui/alert";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";

/** The small round "i" that reveals an explainer next to a label. */
export function InfoButton({
  onClick,
  title,
  pressed,
}: {
  onClick: () => void;
  title?: string;
  pressed: boolean;
}) {
  return (
    <button
      type="button"
      title={title}
      aria-expanded={pressed}
      onClick={(e) => {
        e.stopPropagation();
        onClick();
      }}
      className={cn(
        "flex size-[17px] shrink-0 cursor-pointer items-center justify-center rounded-full border border-[#d1d5db] bg-white p-0",
        "font-mono text-[11px] leading-none font-bold text-muted-foreground",
        "hover:border-brand hover:text-brand-ink",
      )}
    >
      i
    </button>
  );
}

/** Label (+ optional explainer) above a control, with messages underneath. */
export function Field({
  label,
  htmlFor,
  info,
  infoTitle,
  messages,
  className,
  children,
}: {
  label: string;
  htmlFor?: string;
  info?: ReactNode;
  infoTitle?: string;
  messages?: ReactNode;
  className?: string;
  children: ReactNode;
}) {
  const [showInfo, setShowInfo] = useState(false);
  return (
    <div className={cn("min-w-0", className)}>
      <div className="mb-2 flex items-center gap-[7px]">
        <Label htmlFor={htmlFor}>{label}</Label>
        {info ? (
          <InfoButton
            title={infoTitle}
            pressed={showInfo}
            onClick={() => setShowInfo((v) => !v)}
          />
        ) : null}
      </div>
      {info && showInfo ? <Alert className="mb-2.5">{info}</Alert> : null}
      {children}
      {messages ? (
        <div className="mt-[9px] flex flex-col gap-1 text-[12.5px] leading-[1.5] text-muted-foreground">
          {messages}
        </div>
      ) : null}
    </div>
  );
}
