import { cva, type VariantProps } from "class-variance-authority";
import type * as React from "react";

import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center rounded-full border text-xs font-semibold whitespace-nowrap",
  {
    variants: {
      variant: {
        default: "border-line bg-hair text-muted-foreground",
        ok: "border-brand-line bg-brand-bg text-brand-ink",
        bad: "border-danger-line bg-danger-bg text-danger-ink",
        warn: "border-warn-bg bg-warn-bg text-warn-ink",
        plain: "border-transparent bg-hair text-muted-foreground",
        plainOk: "border-transparent bg-brand-bg text-brand-ink",
      },
      size: {
        default: "px-[11px] py-[5px]",
        sm: "px-2.5 py-1 text-[11.5px]",
      },
    },
    defaultVariants: { variant: "default", size: "default" },
  },
);

function Badge({
  className,
  variant,
  size,
  ...props
}: React.ComponentProps<"span"> & VariantProps<typeof badgeVariants>) {
  return (
    <span
      data-slot="badge"
      className={cn(badgeVariants({ variant, size, className }))}
      {...props}
    />
  );
}

export { Badge, badgeVariants };
