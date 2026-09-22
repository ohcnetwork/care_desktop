import { Plus, X } from "lucide-react";
import { useState, type KeyboardEvent } from "react";

import {
  ADMINISTRATION,
  createDepartment,
  createDepartmentLocation,
  ensureLocationOrganization,
  listDepartments,
  listLocations,
} from "@/care/departments";
import { sameName } from "@/care/organizations";
import { BatchPanel } from "@/components/batch-panel";
import { Screen, ScreenBody, ScreenHead } from "@/components/screen";
import { StepFoot } from "@/components/step-foot";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { runBatch, type BatchProgress } from "@/lib/batch";
import { errorText } from "@/lib/format";
import { useWizard, type Department } from "@/state/wizard";

const SUGGESTIONS = ["General Medicine", "Paediatrics", "Laboratory", "Pharmacy", "Radiology", "Nursing"];

export function DepartmentsStep() {
  const { progress, complete, skip, update } = useWizard();
  const [names, setNames] = useState<string[]>(progress.departments.map((d) => d.name));
  const [draft, setDraft] = useState("");
  const [busy, setBusy] = useState(false);
  const [batch, setBatch] = useState<BatchProgress | null>(null);
  const [problem, setProblem] = useState("");
  const [created, setCreated] = useState<Department[] | null>(null);

  const add = (raw: string) => {
    const name = raw.trim().replace(/\s+/g, " ");
    if (!name || names.some((n) => sameName(n, name)) || sameName(name, ADMINISTRATION)) return;
    setNames((list) => [...list, name]);
    setDraft("");
  };

  const onKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      add(draft);
    }
  };

  const run = async () => {
    setBusy(true);
    setProblem("");
    setCreated(null);
    try {
      const facilityId = progress.facilityId;
      const existing = await listDepartments(facilityId);
      const locations = await listLocations(facilityId);
      const administration = existing.find((o) => sameName(o.name, ADMINISTRATION));
      const result: Department[] = [];
      const report = await runBatch(
        names,
        (n) => n,
        async (name) => {
          let org = existing.find((o) => sameName(o.name, name));
          let outcome: "created" | "skipped" = "skipped";
          if (!org) {
            org = await createDepartment(facilityId, name);
            outcome = "created";
          }
          let location = locations.find((l) => sameName(l.name, name));
          if (!location) {
            location = await createDepartmentLocation(facilityId, name, [org.id]);
            outcome = "created";
          }
          if (await ensureLocationOrganization(facilityId, location.id, org.id)) outcome = "created";
          if (administration) {
            await ensureLocationOrganization(facilityId, location.id, administration.id).catch(() => false);
          }
          result.push({ name, organizationId: org.id, locationId: location.id });
          return outcome;
        },
        setBatch,
        1,
      );
      result.sort((a, b) => names.indexOf(a.name) - names.indexOf(b.name));
      update({ departments: result, administrationId: administration?.id ?? "" });
      if (report.failed === 0) setCreated(result);
    } catch (e) {
      setProblem(errorText(e));
    } finally {
      setBusy(false);
    }
  };

  const finished = created !== null;

  return (
    <Screen>
      <ScreenHead
        kicker="Step 4"
        title="Departments"
        subtitle="Each department also gets a location of the same name, managed by that department, so beds and rooms can be placed under it later. Administration is created with the facility."
      />
      <ScreenBody>
        <div className="flex max-w-[640px] flex-col gap-4">
          <div className="rounded-xl border border-line bg-white p-3.5">
            {names.length ? (
              <ul className="mb-3 flex flex-col gap-2">
                {names.map((n) => (
                  <li key={n} className="flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm">
                    <span className="flex-1">{n}</span>
                    {!created ? (
                      <button
                        type="button"
                        aria-label={`Remove ${n}`}
                        className="text-faint hover:text-danger-ink"
                        onClick={() => setNames((list) => list.filter((x) => x !== n))}
                      >
                        <X className="size-4" />
                      </button>
                    ) : null}
                  </li>
                ))}
              </ul>
            ) : (
              <div className="mb-3 text-[13px] text-muted-foreground">No departments added yet.</div>
            )}
            {!created ? (
              <div className="flex gap-2">
                <Input
                  value={draft}
                  placeholder="e.g. ENT"
                  onChange={(e) => setDraft(e.target.value)}
                  onKeyDown={onKey}
                />
                <Button type="button" onClick={() => add(draft)} disabled={!draft.trim()}>
                  <Plus className="size-4" /> Add
                </Button>
              </div>
            ) : null}
          </div>
          {!created ? (
            <div className="flex flex-wrap items-center gap-2 text-[12.5px] text-muted-foreground">
              <span>Common:</span>
              {SUGGESTIONS.filter((s) => !names.some((n) => sameName(n, s))).map((s) => (
                <button
                  key={s}
                  type="button"
                  className="rounded-full border border-line bg-white px-2.5 py-1 hover:border-brand hover:text-brand-ink"
                  onClick={() => add(s)}
                >
                  {s}
                </button>
              ))}
            </div>
          ) : null}
          {batch ? <BatchPanel title="Departments and locations" progress={batch} running={busy} /> : null}
          {problem ? <Alert variant="danger">{problem}</Alert> : null}
        </div>
      </ScreenBody>
      <StepFoot
        primary={finished ? "Continue" : "Create departments"}
        primaryDisabled={!finished && names.length === 0}
        onPrimary={finished ? () => complete("departments") : () => void run()}
        busy={busy}
        onSkip={created ? undefined : () => skip("departments")}
      />
    </Screen>
  );
}
