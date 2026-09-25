import { useCallback, useState } from "react";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { CheckRows } from "@/screens/setup/check-rows";
import { useRequirementChecks } from "@/screens/setup/use-requirement-checks";
import { useCare } from "@/state/care-store";

/**
 * The wizard's check list, pointed at a clinic that has already been set up and
 * has stopped answering. Same rows, same fix buttons - what breaks a running
 * clinic is what stopped it from starting in the first place: Docker not up, the
 * network gone Public after a new router, the stack not running.
 *
 * Mounted only while open, so the checks run when the operator asks and the panel
 * is not spawning `docker` and PowerShell in the background all day.
 */
export function TroubleDialog({ onClose }: { onClose: () => void }) {
  const { mdnsName, refresh } = useCare();
  const { checks, checking, recheckAll } = useRequirementChecks(mdnsName, "running");
  const [fixing, setFixing] = useState(false);

  const recheck = useCallback(async () => {
    await recheckAll();
    await refresh(); // the panel's own state comes from a different poll
  }, [recheckAll, refresh]);

  return (
    <AlertDialog open>
      <AlertDialogContent className="max-w-[600px]">
        <AlertDialogTitle>Staff can&apos;t reach the clinic</AlertDialogTitle>
        <AlertDialogDescription>
          These are the things the clinic needs on this computer. Anything marked Not ready has a
          button that fixes it.
        </AlertDialogDescription>

        <div className="mt-3">
          <CheckRows
            checks={checks}
            onDone={() => void recheck()}
            locked={checking}
            onBusyChange={setFixing}
          />
        </div>

        <AlertDialogFooter>
          <AlertDialogCancel disabled={fixing} onClick={onClose}>
            Close
          </AlertDialogCancel>
          <AlertDialogAction
            disabled={checking || fixing}
            onClick={(e) => {
              e.preventDefault();
              void recheck();
            }}
          >
            {checking ? "Checking…" : "Check again"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
