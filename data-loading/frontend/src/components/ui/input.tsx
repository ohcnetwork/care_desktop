import * as React from "react";

import { cn } from "@/lib/utils";

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        "h-[42px] w-full min-w-0 rounded-md border border-line bg-white px-3 text-sm text-ink outline-none transition-colors",
        "placeholder:text-faint focus-visible:border-brand",
        "disabled:cursor-not-allowed disabled:bg-background disabled:text-faint",
        "aria-invalid:border-danger-line",
        className,
      )}
      {...props}
    />
  );
}

export { Input };
