import type { ReactNode } from "react";

import { FootNote, ScreenFoot } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/spinner";

export function StepFoot({
  primary,
  onPrimary,
  primaryDisabled,
  busy,
  onSkip,
  onBack,
  note,
}: {
  primary: string;
  onPrimary: () => void;
  primaryDisabled?: boolean;
  busy?: boolean;
  onSkip?: () => void;
  onBack?: () => void;
  note?: ReactNode;
}) {
  return (
    <ScreenFoot>
      {onBack ? (
        <Button disabled={busy} onClick={onBack}>
          Back
        </Button>
      ) : null}
      <Button variant="primary" size="lg" disabled={primaryDisabled || busy} onClick={onPrimary}>
        {busy ? <Spinner /> : null}
        {primary}
      </Button>
      {onSkip ? (
        <Button variant="ghost" disabled={busy} onClick={onSkip}>
          Skip for now
        </Button>
      ) : null}
      <div className="flex-1" />
      {note ? <FootNote>{note}</FootNote> : null}
    </ScreenFoot>
  );
}
