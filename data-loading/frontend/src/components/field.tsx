import type { ReactNode } from "react";

import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";

export function Field({
  label,
  htmlFor,
  required,
  hint,
  error,
  className,
  children,
}: {
  label: string;
  htmlFor?: string;
  required?: boolean;
  hint?: ReactNode;
  error?: string;
  className?: string;
  children: ReactNode;
}) {
  return (
    <div className={cn("min-w-0", className)}>
      <Label htmlFor={htmlFor} className="mb-2 block">
        {label}
        {required ? <span className="ml-0.5 text-danger-ink">*</span> : null}
      </Label>
      {children}
      {error ? (
        <div className="mt-[7px] text-[12.5px] leading-[1.5] text-danger-ink">{error}</div>
      ) : hint ? (
        <div className="mt-[7px] text-[12.5px] leading-[1.5] text-muted-foreground">{hint}</div>
      ) : null}
    </div>
  );
}
