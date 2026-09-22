import { useState } from "react";

import statesData from "../../../data/states-and-districts.json";
import {
  createDistrict,
  createRoleOrganization,
  createState,
  listAllDistricts,
  listRoleOrganizations,
  listStates,
  ROLE_ORGANIZATIONS,
  sameName,
} from "@/care/organizations";
import { BatchPanel } from "@/components/batch-panel";
import { Screen, ScreenBody, ScreenHead } from "@/components/screen";
import { StepFoot } from "@/components/step-foot";
import { Alert } from "@/components/ui/alert";
import { runBatch, type BatchProgress } from "@/lib/batch";
import { errorText, plural } from "@/lib/format";
import { useWizard } from "@/state/wizard";

type StateRow = { state: string; slug: string; districts: string };

const STATES = (statesData as StateRow[]).map((s) => ({
  name: s.state.trim(),
  districts: s.districts.split(",").map((d) => d.trim()).filter(Boolean),
}));
const DISTRICT_COUNT = STATES.reduce((n, s) => n + s.districts.length, 0);

const key = (parent: string, name: string) => `${parent}::${name.trim().toLowerCase()}`;

export function StatesStep() {
  const { complete, update } = useWizard();
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState("");
  const [roles, setRoles] = useState<BatchProgress | null>(null);
  const [states, setStates] = useState<BatchProgress | null>(null);
  const [districts, setDistricts] = useState<BatchProgress | null>(null);
  const [finished, setFinished] = useState(false);

  const load = async () => {
    setBusy(true);
    setProblem("");
    setFinished(false);
    try {
      const existingRoles = await listRoleOrganizations();
      const roleIds: Record<string, string> = {};
      for (const org of existingRoles) roleIds[org.name] = org.id;
      await runBatch(
        ROLE_ORGANIZATIONS,
        (name) => name,
        async (name) => {
          const found = existingRoles.find((o) => sameName(o.name, name));
          if (found) return "skipped";
          const created = await createRoleOrganization(name);
          roleIds[name] = created.id;
          return "created";
        },
        setRoles,
        2,
      );
      update({ roleOrganizations: roleIds });

      const existingStates = await listStates();
      const stateIds = new Map<string, string>();
      for (const s of existingStates) stateIds.set(s.name.trim().toLowerCase(), s.id);
      const stateReport = await runBatch(
        STATES,
        (s) => s.name,
        async (s) => {
          if (stateIds.has(s.name.toLowerCase())) return "skipped";
          const created = await createState(s.name);
          stateIds.set(s.name.toLowerCase(), created.id);
          return "created";
        },
        setStates,
        2,
      );
      if (stateReport.failed) throw new Error("Some states could not be created; fix the errors and try again.");

      const existingDistricts = await listAllDistricts();
      const districtKeys = new Set(existingDistricts.map((d) => key(d.parent?.id ?? "", d.name)));
      const pairs = STATES.flatMap((s) => s.districts.map((d) => ({ state: s.name, district: d })));
      await runBatch(
        pairs,
        (p) => `${p.district} (${p.state})`,
        async (p) => {
          const stateId = stateIds.get(p.state.toLowerCase());
          if (!stateId) throw new Error(`state ${p.state} is missing`);
          if (districtKeys.has(key(stateId, p.district))) return "skipped";
          await createDistrict(stateId, p.district);
          return "created";
        },
        setDistricts,
        4,
      );
      setFinished(true);
    } catch (e) {
      setProblem(errorText(e));
    } finally {
      setBusy(false);
    }
  };

  const complete_ = () => complete("states");

  return (
    <Screen>
      <ScreenHead
        kicker="Step 1"
        title="States and districts"
        subtitle={`Loads all ${STATES.length} Indian states and union territories and their ${DISTRICT_COUNT} districts, plus the staff role groups. Anything already present is left alone.`}
      />
      <ScreenBody>
        <div className="flex max-w-[640px] flex-col gap-3.5">
          {roles ? <BatchPanel title="Role groups" progress={roles} running={busy && !states} /> : null}
          {states ? <BatchPanel title="States" progress={states} running={busy && !districts} /> : null}
          {districts ? <BatchPanel title="Districts" progress={districts} running={busy && !finished} /> : null}
          {!roles ? (
            <Alert>
              This takes a minute or two. Keep this page open until it finishes; if it is interrupted,
              press the button again and it carries on from where it stopped.
            </Alert>
          ) : null}
          {problem ? <Alert variant="danger">{problem}</Alert> : null}
          {finished ? (
            <Alert>
              Done. {plural(states?.created ?? 0, "state")} and {plural(districts?.created ?? 0, "district")} were
              added.
            </Alert>
          ) : null}
        </div>
      </ScreenBody>
      <StepFoot
        primary={finished ? "Continue" : roles ? "Load again" : "Load states and districts"}
        onPrimary={finished ? complete_ : load}
        busy={busy}
        note={finished ? undefined : "Nothing is changed in CARE until you press the button."}
      />
    </Screen>
  );
}
