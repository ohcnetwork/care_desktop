import { cn } from "@/lib/utils";

export function Spinner({ className }: { className?: string }) {
  return (
    <span
      role="status"
      aria-label="Working"
      className={cn(
        "block size-[17px] animate-spin rounded-full border-2 border-current border-t-transparent",
        className,
      )}
    />
  );
}
