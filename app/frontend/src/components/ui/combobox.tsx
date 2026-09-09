// A shadcn/ui-style Combobox: Popover + cmdk, i.e. a Select with a search box.
// Earned by the facility type, whose 18 options include labels as long as
// "Non Clinical Non Governmental Organization" — typing "gov" narrows it to two.
import { Check, ChevronDown } from "lucide-react";
import { useState } from "react";

import {
  Command,
  CommandEmpty,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { cn } from "@/lib/utils";
import type { Option } from "@/options";

export type ComboboxProps = {
  value: string;
  onChange: (value: string) => void;
  options: Option[];
  placeholder?: string;
  searchPlaceholder?: string;
  emptyText?: string;
  "aria-label"?: string;
  id?: string;
};

export function Combobox({
  value,
  onChange,
  options,
  placeholder = "Select…",
  searchPlaceholder = "Search…",
  emptyText = "No match.",
  id,
  ...props
}: ComboboxProps) {
  const [open, setOpen] = useState(false);
  const selected = options.find((o) => o.value === value);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        id={id}
        role="combobox"
        aria-expanded={open}
        aria-label={props["aria-label"]}
        className={cn(
          "flex h-[42px] w-full min-w-0 cursor-pointer items-center justify-between gap-2 rounded-md",
          "border border-line bg-white px-3 text-left text-sm leading-none text-ink outline-none transition-colors",
          "hover:border-faint focus-visible:border-brand data-[state=open]:border-brand",
        )}
      >
        <span className={cn("truncate", selected ? "text-ink" : "text-faint")}>
          {selected ? selected.label : placeholder}
        </span>
        <ChevronDown className="size-3.5 shrink-0 text-faint" strokeWidth={2.2} />
      </PopoverTrigger>
      <PopoverContent className="w-[var(--radix-popover-trigger-width)] p-0">
        {/* cmdk owns the filtering and the up/down + Enter handling; it filters on
            each item's `value`, so we give it the label and map back below. */}
        <Command>
          <CommandInput placeholder={searchPlaceholder} />
          <CommandList>
            <CommandEmpty>{emptyText}</CommandEmpty>
            {options.map((o) => (
              <CommandItem
                key={o.value}
                value={o.label}
                onSelect={() => {
                  onChange(o.value);
                  setOpen(false);
                }}
              >
                <span className="truncate">{o.label}</span>
                {o.value === value ? (
                  <Check className="size-3.5 shrink-0" strokeWidth={2.4} />
                ) : null}
              </CommandItem>
            ))}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
