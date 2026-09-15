import * as RadioGroupPrimitive from "@radix-ui/react-radio-group";
import type * as React from "react";

import { cn } from "@/lib/utils";

function RadioGroup({
  className,
  ...props
}: React.ComponentProps<typeof RadioGroupPrimitive.Root>) {
  return (
    <RadioGroupPrimitive.Root
      data-slot="radio-group"
      className={cn("flex flex-wrap gap-2", className)}
      {...props}
    />
  );
}

function RadioGroupItem({
  className,
  ...props
}: React.ComponentProps<typeof RadioGroupPrimitive.Item>) {
  return (
    <RadioGroupPrimitive.Item
      data-slot="radio-group-item"
      className={cn(
        "flex size-4 shrink-0 cursor-pointer items-center justify-center rounded-full border border-[#d1d5db] bg-white outline-none transition-colors",
        "focus-visible:ring-2 focus-visible:ring-ring/40",
        "data-[state=checked]:border-brand",
        "disabled:cursor-not-allowed disabled:opacity-60",
        className,
      )}
      {...props}
    >
      <RadioGroupPrimitive.Indicator className="size-2 rounded-full bg-brand" />
    </RadioGroupPrimitive.Item>
  );
}

/**
 * A whole option as one click target: radio + label in a bordered chip, which
 * turns green when chosen. Used for every yes/no and short-list setting.
 */
function RadioChip({
  className,
  children,
  ...props
}: React.ComponentProps<typeof RadioGroupPrimitive.Item>) {
  return (
    <label
      className={cn(
        "flex cursor-pointer items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-[13px] font-medium text-ink2 transition-colors select-none",
        "hover:border-faint has-[[data-state=checked]]:border-brand-line has-[[data-state=checked]]:bg-brand-bg has-[[data-state=checked]]:text-brand-ink",
        "has-[:disabled]:cursor-not-allowed has-[:disabled]:opacity-60",
        className,
      )}
    >
      <RadioGroupItem {...props} />
      <span>{children}</span>
    </label>
  );
}

export { RadioGroup, RadioGroupItem, RadioChip };
