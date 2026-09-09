import { useState } from "react";

import { Spinner } from "@/components/spinner";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { bridge } from "@/lib/bridge";
import { errorText } from "@/lib/format";
import type { RestartPlan } from "@/types";

/**
 * Offered only when the host says a restart is genuinely outstanding, and only
 * straight after an install. "Later" is a real choice: the operator keeps a
 * working wizard and can finish when the clinic is closed.
 */
export function RestartDialog({
  plan,
  onDismiss,
}: {
  plan: RestartPlan | null;
  onDismiss: () => void;
}) {
  const [restarting, setRestarting] = useState(false);
  const [failure, setFailure] = useState("");

  return (
    <AlertDialog open={plan !== null}>
      <AlertDialogContent>
        <AlertDialogTitle>{plan?.title}</AlertDialogTitle>
        <AlertDialogDescription>{plan?.detail}</AlertDialogDescription>
        {failure ? (
          <div className="mt-3 text-[12.5px] leading-[1.5] text-danger-ink">
            {failure} You can restart the computer yourself instead.
          </div>
        ) : null}
        <AlertDialogFooter>
          <AlertDialogCancel disabled={restarting} onClick={onDismiss}>
            I'll restart later
          </AlertDialogCancel>
          <AlertDialogAction
            disabled={restarting}
            onClick={(e) => {
              // Keep the dialog up: the machine is about to go down, and a
              // closing dialog would look like the restart was cancelled.
              e.preventDefault();
              setRestarting(true);
              setFailure("");
              void bridge.RestartNow().catch((err) => {
                setFailure(errorText(err));
                setRestarting(false);
              });
            }}
          >
            {restarting ? <Spinner className="size-3.5" /> : null}
            {restarting ? "Restarting…" : (plan?.label ?? "Restart now")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
