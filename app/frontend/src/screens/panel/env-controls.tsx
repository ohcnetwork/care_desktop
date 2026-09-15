// One control per kind of setting. Each takes the raw env value (undefined when
// the key is not in the file) and reports the raw value back; the friendly
// labels live in env-schema.ts, so nothing here knows what a key means.
import { useState } from "react";

import { Input } from "@/components/ui/input";
import { RadioChip, RadioGroup } from "@/components/ui/radio-group";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { cn } from "@/lib/utils";
import type { Option, Setting } from "./env-schema";

type ControlProps<K extends Setting["kind"]> = {
  setting: Extract<Setting, { kind: K }>;
  value: string | undefined;
  onChange: (value: string | undefined) => void;
  disabled: boolean;
  invalid?: boolean;
};

const BOX = "h-9 rounded-[8px] px-[11px] text-[13px]";

export function RadioControl({ setting, value, onChange, disabled }: ControlProps<"radio">) {
  return (
    <RadioGroup
      value={value ?? setting.fallback}
      onValueChange={onChange}
      disabled={disabled}
      aria-label={setting.label}
      className="@lg:justify-end"
    >
      {setting.options.map((o) => (
        <RadioChip key={o.value} value={o.value}>
          {o.label}
        </RadioChip>
      ))}
    </RadioGroup>
  );
}

// Radix Select cannot carry an empty-string item, so "not set" travels as NONE.
const NONE = "\0none";

export function SelectControl({ setting, value, onChange, disabled }: ControlProps<"select">) {
  const current = value ?? setting.fallback;
  return (
    <Select
      value={current === "" ? NONE : current}
      onValueChange={(v) => onChange(v === NONE ? "" : v)}
      disabled={disabled}
    >
      <SelectTrigger className={cn(BOX, "w-full")} aria-label={setting.label}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {setting.none ? <SelectItem value={NONE}>{setting.none}</SelectItem> : null}
        {setting.options.map((o) => (
          <SelectItem key={o.value} value={o.value}>
            {o.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

/** Comma-separated list of codes shown as pressable chips, in option order. */
export function MultiControl({ setting, value, onChange, disabled, invalid }: ControlProps<"multi">) {
  const chosen = value === undefined ? setting.fallback : splitList(value);
  return (
    <ToggleGroup
      type="multiple"
      value={chosen}
      onValueChange={(next) => onChange(joinList(next, setting.options))}
      disabled={disabled}
      aria-label={setting.label}
      aria-invalid={invalid || undefined}
      className={cn("@lg:justify-end", invalid && "rounded-md ring-2 ring-danger-line")}
    >
      {setting.options.map((o) => (
        <ToggleGroupItem key={o.value} value={o.value}>
          {o.label}
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  );
}

export function splitList(value: string): string[] {
  return value.split(",").map((s) => s.trim()).filter(Boolean);
}

function joinList(chosen: string[], options: Option[]): string {
  return options.filter((o) => chosen.includes(o.value)).map((o) => o.value).join(",");
}

export function IntControl({ setting, value, onChange, disabled, invalid }: ControlProps<"int">) {
  return (
    <div className="flex items-center gap-2">
      <Input
        type="number"
        inputMode="numeric"
        className={cn(BOX, "w-[120px] text-right")}
        placeholder={setting.fallback ?? ""}
        min={setting.min}
        max={setting.max}
        step={1}
        value={value ?? ""}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        aria-label={setting.label}
        aria-invalid={invalid || undefined}
      />
      {setting.unit ? (
        <span className="text-[13px] text-muted-foreground">{setting.unit}</span>
      ) : null}
    </div>
  );
}

export function TextControl({ setting, value, onChange, disabled }: ControlProps<"text" | "secret">) {
  const [reveal, setReveal] = useState(false);
  const secret = setting.kind === "secret";
  return (
    <div className="relative w-full">
      <Input
        type={secret && !reveal ? "password" : "text"}
        className={cn(BOX, "w-full", secret && "pr-14")}
        placeholder={setting.placeholder ?? (secret ? "Not set" : "")}
        autoComplete="off"
        spellCheck={false}
        value={value ?? ""}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        aria-label={setting.label}
      />
      {secret ? (
        <button
          type="button"
          onClick={() => setReveal((v) => !v)}
          className="absolute top-1/2 right-2.5 -translate-y-1/2 cursor-pointer text-xs font-semibold text-muted-foreground hover:text-brand-ink"
        >
          {reveal ? "Hide" : "Show"}
        </button>
      ) : null}
    </div>
  );
}

// REACT_MAIN_LOGO is {"light":url,"dark":url}. Two address boxes. A side left
// empty is filled from the other when saved (see normaliseLogo), not while
// typing, so the second box does not echo the first keystroke by keystroke.
type Logo = { light: string; dark: string };

function parseLogo(value: string | undefined): Logo {
  if (!value) return { light: "", dark: "" };
  try {
    const o = JSON.parse(value) as Partial<Logo>;
    return { light: o.light ?? "", dark: o.dark ?? "" };
  } catch {
    return { light: "", dark: "" };
  }
}

export function normaliseLogo(value: string): string {
  const { light, dark } = parseLogo(value);
  const l = light.trim();
  const d = dark.trim();
  if (!l && !d) return "";
  return JSON.stringify({ light: l || d, dark: d || l });
}

export function LogoControl({ setting, value, onChange, disabled }: ControlProps<"logo">) {
  const logo = parseLogo(value);
  const patch = (next: Partial<Logo>) => {
    const merged = { ...logo, ...next };
    onChange(merged.light.trim() || merged.dark.trim() ? JSON.stringify(merged) : "");
  };
  return (
    <div className="flex w-full flex-col gap-2">
      <Input
        className={cn(BOX, "w-full")}
        placeholder="Light mode: https://…/logo.svg"
        spellCheck={false}
        value={logo.light}
        onChange={(e) => patch({ light: e.target.value })}
        disabled={disabled}
        aria-label={`${setting.label}, light mode`}
      />
      <Input
        className={cn(BOX, "w-full")}
        placeholder="Dark mode: leave empty to reuse the light one"
        spellCheck={false}
        value={logo.dark}
        onChange={(e) => patch({ dark: e.target.value })}
        disabled={disabled}
        aria-label={`${setting.label}, dark mode`}
      />
    </div>
  );
}

export function SettingControl(props: {
  setting: Setting;
  value: string | undefined;
  onChange: (value: string | undefined) => void;
  disabled: boolean;
  invalid?: boolean;
}) {
  const { setting } = props;
  switch (setting.kind) {
    case "radio":
      return <RadioControl {...props} setting={setting} />;
    case "select":
      return <SelectControl {...props} setting={setting} />;
    case "multi":
      return <MultiControl {...props} setting={setting} />;
    case "int":
      return <IntControl {...props} setting={setting} />;
    case "text":
    case "secret":
      return <TextControl {...props} setting={setting} />;
    case "logo":
      return <LogoControl {...props} setting={setting} />;
  }
}
