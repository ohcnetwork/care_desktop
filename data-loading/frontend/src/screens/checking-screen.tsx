import { useEffect, useState } from "react";

import { listFacilities } from "@/care/facility";
import { Screen, ScreenBody, ScreenHead } from "@/components/screen";
import { Spinner } from "@/components/spinner";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { errorText } from "@/lib/format";
import { useWizard } from "@/state/wizard";

export function CheckingScreen() {
  const { progress, reset, setPhase, setExistingFacility } = useWizard();
  const [problem, setProblem] = useState("");
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setProblem("");
    (async () => {
      try {
        const facilities = await listFacilities();
        if (cancelled) return;
        const resumable = progress.facilityId && facilities.some((f) => f.id === progress.facilityId);
        if (resumable) {
          setPhase("wizard");
        } else if (facilities.length > 0) {
          setExistingFacility(facilities.map((f) => f.name).join(", "));
          setPhase("already-set-up");
        } else {
          if (progress.facilityId) reset();
          setPhase("wizard");
        }
      } catch (e) {
        if (!cancelled) setProblem(errorText(e));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [attempt, progress.facilityId, reset, setPhase, setExistingFacility]);

  return (
    <Screen>
      <ScreenHead title="Checking this instance" subtitle="Looking for an existing facility." />
      <ScreenBody>
        {problem ? (
          <div className="flex max-w-[520px] flex-col gap-4">
            <Alert variant="danger">{problem}</Alert>
            <div>
              <Button onClick={() => setAttempt((n) => n + 1)}>Try again</Button>
            </div>
          </div>
        ) : (
          <div className="flex items-center gap-3 text-muted-foreground">
            <Spinner /> Checking…
          </div>
        )}
      </ScreenBody>
    </Screen>
  );
}
