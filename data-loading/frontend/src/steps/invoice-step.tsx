import { useState } from "react";

import { setInvoiceExpression } from "@/care/facility";
import { Field } from "@/components/field";
import { Screen, ScreenBody, ScreenHead } from "@/components/screen";
import { StepFoot } from "@/components/step-foot";
import { Alert } from "@/components/ui/alert";
import { Input } from "@/components/ui/input";
import { cleanInitials, invoiceExpression, previewInvoice } from "@/lib/expressions";
import { errorText } from "@/lib/format";
import { useWizard } from "@/state/wizard";

export function InvoiceStep() {
  const { progress, complete, skip, update } = useWizard();
  const [initials, setInitials] = useState(progress.initials);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState("");
  const [finished, setFinished] = useState(false);

  const save = async () => {
    setBusy(true);
    setProblem("");
    try {
      await setInvoiceExpression(progress.facilityId, invoiceExpression(initials));
      update({ initials });
      setFinished(true);
    } catch (e) {
      setProblem(errorText(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Screen>
      <ScreenHead
        kicker="Step 7"
        title="Invoice numbers"
        subtitle="Every invoice gets the facility's initials followed by a running number."
      />
      <ScreenBody>
        <div className="flex max-w-[480px] flex-col gap-5">
          <Field
            label="Facility initials"
            htmlFor="initials"
            required
            hint={`Taken from "${progress.facilityName}". Letters and digits only.`}
          >
            <Input
              id="initials"
              value={initials}
              disabled={finished}
              className="max-w-[200px] font-mono uppercase"
              onChange={(e) => setInitials(cleanInitials(e.target.value))}
            />
          </Field>
          <div className="rounded-xl border border-line bg-white px-4 py-3.5">
            <div className="text-[11px] font-semibold tracking-[0.04em] text-faint uppercase">Preview</div>
            <div className="mt-1 font-mono text-[15px] font-semibold text-brand-ink">
              {initials ? previewInvoice(initials, 0) : "—"}, {initials ? previewInvoice(initials, 1) : "—"}, …
            </div>
            <div className="mt-2 text-[12.5px] text-muted-foreground">
              Saved as <span className="font-mono">{invoiceExpression(initials || "ABC")}</span>. It can be
              changed later under Billing settings in CARE.
            </div>
          </div>
          {problem ? <Alert variant="danger">{problem}</Alert> : null}
          {finished ? <Alert>Saved.</Alert> : null}
        </div>
      </ScreenBody>
      <StepFoot
        primary={finished ? "Continue" : "Save"}
        primaryDisabled={!finished && initials.length < 2}
        onPrimary={finished ? () => complete("invoice", { initials }) : () => void save()}
        busy={busy}
        onSkip={finished ? undefined : () => skip("invoice")}
      />
    </Screen>
  );
}
