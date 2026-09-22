import { useState } from "react";

import { createIdentifierConfig, listIdentifierConfigs, PATIENT_ID_SYSTEM } from "@/care/identifiers";
import { Field } from "@/components/field";
import { Screen, ScreenBody, ScreenHead } from "@/components/screen";
import { StepFoot } from "@/components/step-foot";
import { Alert } from "@/components/ui/alert";
import { Input } from "@/components/ui/input";
import { cleanInitials, patientIdExpression, previewPatientId } from "@/lib/expressions";
import { errorText } from "@/lib/format";
import { useWizard } from "@/state/wizard";

export function PatientIdStep() {
  const { progress, complete, skip, update } = useWizard();
  const [initials, setInitials] = useState(progress.initials);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState("");
  const [finished, setFinished] = useState("");

  const save = async () => {
    setBusy(true);
    setProblem("");
    try {
      const existing = await listIdentifierConfigs();
      const already = existing.find((c) => c.config.system === PATIENT_ID_SYSTEM && c.status === "active");
      if (!already) {
        await createIdentifierConfig({
          display: "Patient ID",
          default_value: patientIdExpression(initials),
        });
        setFinished("Saved.");
      } else {
        setFinished("A patient ID format already exists on this instance, so nothing was changed.");
      }
      update({ initials });
    } catch (e) {
      setProblem(errorText(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Screen>
      <ScreenHead
        kicker="Step 8"
        title="Patient IDs"
        subtitle="The number printed on cards and reports, given automatically when a patient is registered. One series is shared across the whole instance."
      />
      <ScreenBody>
        <div className="flex max-w-[480px] flex-col gap-5">
          <Field label="Prefix" htmlFor="pinitials" required hint="Usually the facility initials.">
            <Input
              id="pinitials"
              value={initials}
              disabled={!!finished}
              className="max-w-[200px] font-mono uppercase"
              onChange={(e) => setInitials(cleanInitials(e.target.value))}
            />
          </Field>
          <div className="rounded-xl border border-line bg-white px-4 py-3.5">
            <div className="text-[11px] font-semibold tracking-[0.04em] text-faint uppercase">Preview</div>
            <div className="mt-1 font-mono text-[15px] font-semibold text-brand-ink">
              {initials ? previewPatientId(initials, 0) : "—"}, {initials ? previewPatientId(initials, 1) : "—"}, …
            </div>
            <div className="mt-2 text-[12.5px] text-muted-foreground">
              Saved as <span className="font-mono">{patientIdExpression(initials || "ABC")}</span>. Editable
              later under Patient identifier settings in CARE.
            </div>
          </div>
          {problem ? <Alert variant="danger">{problem}</Alert> : null}
          {finished ? <Alert>{finished}</Alert> : null}
        </div>
      </ScreenBody>
      <StepFoot
        primary={finished ? "Continue" : "Save"}
        primaryDisabled={!finished && initials.length < 2}
        onPrimary={finished ? () => complete("patient-id", { initials }) : () => void save()}
        busy={busy}
        onSkip={finished ? undefined : () => skip("patient-id")}
      />
    </Screen>
  );
}
