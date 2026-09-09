import { useState } from "react";

import { BoxNote, InputBox, type Tone } from "@/components/input-box";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import type { PasswordStrength } from "@/hooks/use-password-strength";

const MISMATCH =
  "The two passwords are different. Retype the confirmation to match exactly, including capital letters.";

const BARE_INPUT =
  "h-full flex-1 rounded-none border-none bg-transparent px-0 focus-visible:border-none";

export type PasswordPairProps = {
  id: string;
  password: string;
  confirm: string;
  onPasswordChange: (value: string) => void;
  onConfirmChange: (value: string) => void;
  strength: PasswordStrength;
};

/** The password + confirmation pair, with the messages the design puts below. */
export function PasswordPair({
  id,
  password,
  confirm,
  onPasswordChange,
  onConfirmChange,
  strength,
}: PasswordPairProps) {
  const [reveal, setReveal] = useState(false);
  const matched = confirm !== "" && confirm === password;

  const passwordTone: Tone = password === "" ? "neutral" : strength.strong ? "ok" : "bad";
  const confirmTone: Tone = confirm === "" ? "neutral" : matched ? "ok" : "bad";

  return (
    <>
      <div className="flex gap-2.5">
        <InputBox tone={passwordTone}>
          <Input
            id={id}
            type={reveal ? "text" : "password"}
            placeholder="Password"
            autoComplete="new-password"
            className={BARE_INPUT}
            value={password}
            onChange={(e) => onPasswordChange(e.target.value)}
          />
          <button
            type="button"
            onClick={() => setReveal((v) => !v)}
            className="cursor-pointer p-1 text-xs font-semibold text-muted-foreground hover:text-brand-ink"
          >
            {reveal ? "Hide" : "Show"}
          </button>
        </InputBox>
        <InputBox tone={confirmTone}>
          <Input
            id={`${id}-confirm`}
            type={reveal ? "text" : "password"}
            placeholder="Confirm password"
            autoComplete="new-password"
            className={BARE_INPUT}
            value={confirm}
            onChange={(e) => onConfirmChange(e.target.value)}
          />
          {confirm === "" ? null : (
            <BoxNote tone={confirmTone}>{matched ? "Match" : "No match"}</BoxNote>
          )}
        </InputBox>
      </div>
      <div className="mt-[9px] flex flex-col gap-1 text-[12.5px] leading-[1.5] text-muted-foreground">
        <span
          className={cn(
            password !== "" && (strength.strong ? "text-brand-ink" : "text-danger-ink"),
          )}
        >
          {strength.message}
        </span>
        {confirm === "" ? null : (
          <span className={matched ? "text-brand-ink" : "text-danger-ink"}>
            {matched ? "Both passwords match." : MISMATCH}
          </span>
        )}
      </div>
    </>
  );
}
