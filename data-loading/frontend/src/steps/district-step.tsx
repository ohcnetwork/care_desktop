import { useEffect, useState } from "react";

import { listDistricts, listStates, type Organization } from "@/care/organizations";
import { Field } from "@/components/field";
import { Screen, ScreenBody, ScreenHead } from "@/components/screen";
import { StepFoot } from "@/components/step-foot";
import { Alert } from "@/components/ui/alert";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { errorText } from "@/lib/format";
import { useWizard } from "@/state/wizard";

export function DistrictStep() {
  const { progress, complete, goTo } = useWizard();
  const [states, setStates] = useState<Organization[]>([]);
  const [districts, setDistricts] = useState<Organization[]>([]);
  const [stateId, setStateId] = useState(progress.stateId);
  const [districtId, setDistrictId] = useState(progress.districtId);
  const [loading, setLoading] = useState(true);
  const [problem, setProblem] = useState("");

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    listStates()
      .then((list) => {
        if (cancelled) return;
        setStates([...list].sort((a, b) => a.name.localeCompare(b.name)));
        setProblem("");
      })
      .catch((e) => !cancelled && setProblem(errorText(e)))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!stateId) {
      setDistricts([]);
      return;
    }
    let cancelled = false;
    listDistricts(stateId)
      .then((list) => {
        if (cancelled) return;
        setDistricts([...list].sort((a, b) => a.name.localeCompare(b.name)));
        setProblem("");
      })
      .catch((e) => !cancelled && setProblem(errorText(e)));
    return () => {
      cancelled = true;
    };
  }, [stateId]);

  const onState = (id: string) => {
    setStateId(id);
    setDistrictId("");
  };

  const state = states.find((s) => s.id === stateId);
  const district = districts.find((d) => d.id === districtId);

  return (
    <Screen>
      <ScreenHead
        kicker="Step 2"
        title="Where is this facility?"
        subtitle="The facility and every staff account are placed under this district."
      />
      <ScreenBody>
        <div className="flex max-w-[480px] flex-col gap-5">
          <Field label="State" htmlFor="state" required>
            <Select value={stateId} onValueChange={onState} disabled={loading}>
              <SelectTrigger id="state">
                <SelectValue placeholder={loading ? "Loading…" : "Select a state"} />
              </SelectTrigger>
              <SelectContent>
                {states.map((s) => (
                  <SelectItem key={s.id} value={s.id}>
                    {s.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
          <Field label="District" htmlFor="district" required>
            <Select value={districtId} onValueChange={setDistrictId} disabled={!stateId}>
              <SelectTrigger id="district">
                <SelectValue placeholder={stateId ? "Select a district" : "Pick a state first"} />
              </SelectTrigger>
              <SelectContent>
                {districts.map((d) => (
                  <SelectItem key={d.id} value={d.id}>
                    {d.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
          {!loading && states.length === 0 ? (
            <Alert variant="danger">No states found. Go back and load the states and districts first.</Alert>
          ) : null}
          {problem ? <Alert variant="danger">{problem}</Alert> : null}
        </div>
      </ScreenBody>
      <StepFoot
        primary="Continue"
        primaryDisabled={!state || !district}
        onPrimary={() =>
          complete("district", {
            stateId,
            stateName: state?.name ?? "",
            districtId,
            districtName: district?.name ?? "",
          })
        }
        onBack={() => goTo("states")}
      />
    </Screen>
  );
}
