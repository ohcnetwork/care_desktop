import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

// The design's button family. `default` is the quiet white/outline button that
// most of the app uses; the rest are the accents it pairs with, including the
// two that sit on the dark green cards (`white`, `glass`).
const buttonVariants = cva(
  "inline-flex shrink-0 items-center justify-center gap-2 rounded-md font-semibold whitespace-nowrap transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring/40 disabled:cursor-not-allowed [&_svg]:pointer-events-none [&_svg]:shrink-0",
  {
    variants: {
      variant: {
        default:
          "border border-line bg-white text-ink2 hover:border-brand hover:text-brand-ink disabled:border-hair disabled:bg-background disabled:text-[#d1d5db]",
        primary:
          "border border-brand bg-brand text-white hover:border-brand-dark hover:bg-brand-dark disabled:border-hair disabled:bg-hair disabled:text-faint disabled:shadow-none",
        destructive:
          "border border-danger bg-danger text-white hover:border-danger-dark hover:bg-danger-dark disabled:border-hair disabled:bg-hair disabled:text-faint",
        soft: "border border-brand-line bg-brand-bg text-brand-ink hover:bg-brand-soft disabled:border-hair disabled:bg-hair disabled:text-faint",
        white: "border border-white bg-white text-brand-deep hover:bg-brand-bg",
        glass:
          "border border-white/20 bg-white/15 text-white hover:border-white/30 hover:bg-white/25",
        outlineGlass:
          "border border-white/20 bg-transparent text-white hover:border-white/30 hover:bg-white/25",
        ghost: "text-muted-foreground hover:text-brand-ink disabled:text-faint",
      },
      size: {
        default: "px-3.5 py-[9px] text-[13px]",
        lg: "rounded-lg px-[22px] py-[13px] text-[14.5px]",
        sm: "h-[30px] rounded-sm px-2.5 text-[12.5px]",
        block: "w-full rounded-md px-4 py-3 text-sm",
        icon: "size-[26px] rounded-sm p-0 text-[13px] leading-none",
        bare: "p-1 text-xs",
      },
    },
    defaultVariants: { variant: "default", size: "default" },
  },
);

function Button({
  className,
  variant,
  size,
  asChild = false,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & { asChild?: boolean }) {
  const Comp = asChild ? Slot : "button";
  return (
    <Comp
      data-slot="button"
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  );
}

export { Button, buttonVariants };
