import { useState } from "react";

import { BatchPanel } from "@/components/batch-panel";
import { Screen, ScreenBody, ScreenHead } from "@/components/screen";
import { StepFoot } from "@/components/step-foot";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import type { BatchProgress } from "@/lib/batch";
import { errorText, plural } from "@/lib/format";
import { CATEGORIES, loadActivityDefinitions } from "@/loaders/activity-definitions";
import { useWizard } from "@/state/wizard";

const ENABLED = new URLSearchParams(window.location.search).has("clinical");

export function ClinicalStep() {
  const { progress, complete, skip } = useWizard();
  const [selected, setSelected] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [batch, setBatch] = useState<BatchProgress | null>(null);
  const [problem, setProblem] = useState("");
  const [finished, setFinished] = useState(false);

  const toggle = (key: string) =>
    setSelected((list) => (list.includes(key) ? list.filter((k) => k !== key) : [...list, key]));

  const run = async () => {
    setBusy(true);
    setProblem("");
    try {
      const report = await loadActivityDefinitions(progress.facilityId, selected, setBatch);
      if (report.failed === 0) setFinished(true);
    } catch (e) {
      setProblem(errorText(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Screen>
      <ScreenHead
        kicker="Step 6"
        title="Clinical definitions"
        subtitle="Orderable tests, scans and procedures, grouped the way the master sheet groups them."
      />
      <ScreenBody>
        <div className="flex max-w-[640px] flex-col gap-4">
          {!ENABLED ? (
            <Alert>
              Loading clinical definitions from this page is not available yet. Add them from the CARE
              settings after setup, or come back to this step when it is switched on.
            </Alert>
          ) : null}
          <div className="flex flex-col gap-2">
            {CATEGORIES.map((c) => {
              const on = selected.includes(c.key);
              return (
                <label
                  key={c.key}
                  className={`flex items-start gap-3 rounded-xl border bg-white px-4 py-3 ${ENABLED && !finished ? "cursor-pointer" : "opacity-70"} ${on ? "border-brand" : "border-line"}`}
                >
                  <Checkbox
                    className="mt-0.5"
                    checked={on}
                    disabled={!ENABLED || finished || busy}
                    onCheckedChange={() => toggle(c.key)}
                  />
                  <span className="min-w-0 flex-1">
                    <span className="flex items-center gap-2">
                      <span className="text-[14px] font-semibold text-ink">{c.name}</span>
                      <Badge size="sm">{plural(c.count, "definition")}</Badge>
                    </span>
                    <span className="mt-0.5 block text-[12.5px] text-muted-foreground">
                      {c.classifications.join(", ")}
                      {c.needs.specimens || c.needs.observations
                        ? ` · needs ${c.needs.specimens} specimen and ${c.needs.observations} observation definitions`
                        : ""}
                      {c.needs.charge_items ? ` · ${c.needs.charge_items} charge items` : ""}
                    </span>
                  </span>
                </label>
              );
            })}
          </div>
          {batch ? <BatchPanel title="Activity definitions" progress={batch} running={busy} /> : null}
          {problem ? <Alert variant="danger">{problem}</Alert> : null}
        </div>
      </ScreenBody>
      <StepFoot
        primary={finished ? "Continue" : "Load selected"}
        primaryDisabled={!finished && (!ENABLED || selected.length === 0)}
        onPrimary={finished ? () => complete("clinical") : () => void run()}
        busy={busy}
        onSkip={finished ? undefined : () => skip("clinical")}
        note={ENABLED ? "Definitions reference specimens, observations and charge items that must already exist." : undefined}
      />
    </Screen>
  );
}
