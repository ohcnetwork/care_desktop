import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from "react";

import type { CurrentUser } from "@/care/auth";

export type StepId =
  | "states"
  | "district"
  | "facility"
  | "departments"
  | "users"
  | "clinical"
  | "invoice"
  | "patient-id"
  | "content"
  | "done";

export const STEPS: { id: StepId; label: string; optional: boolean }[] = [
  { id: "states", label: "States and districts", optional: false },
  { id: "district", label: "Your district", optional: false },
  { id: "facility", label: "Facility", optional: false },
  { id: "departments", label: "Departments", optional: true },
  { id: "users", label: "Users", optional: true },
  { id: "clinical", label: "Clinical definitions", optional: true },
  { id: "invoice", label: "Invoice numbers", optional: true },
  { id: "patient-id", label: "Patient IDs", optional: true },
  { id: "content", label: "Forms and reports", optional: true },
];

export type Department = { name: string; organizationId: string; locationId: string };

export type Progress = {
  version: 1;
  step: StepId;
  done: Partial<Record<StepId, boolean>>;
  skipped: Partial<Record<StepId, boolean>>;
  stateId: string;
  stateName: string;
  districtId: string;
  districtName: string;
  facilityId: string;
  facilityName: string;
  initials: string;
  administrationId: string;
  roleOrganizations: Record<string, string>;
  departments: Department[];
  users: string[];
};

export const EMPTY_PROGRESS: Progress = {
  version: 1,
  step: "states",
  done: {},
  skipped: {},
  stateId: "",
  stateName: "",
  districtId: "",
  districtName: "",
  facilityId: "",
  facilityName: "",
  initials: "",
  administrationId: "",
  roleOrganizations: {},
  departments: [],
  users: [],
};

const STORAGE_KEY = "care-seed-data";

export function loadProgress(): Progress {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return EMPTY_PROGRESS;
    const parsed = JSON.parse(raw) as Partial<Progress>;
    if (parsed.version !== 1) return EMPTY_PROGRESS;
    return { ...EMPTY_PROGRESS, ...parsed };
  } catch {
    return EMPTY_PROGRESS;
  }
}

function saveProgress(p: Progress) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(p));
  } catch {
    return;
  }
}

export function clearProgress() {
  try {
    localStorage.removeItem(STORAGE_KEY);
  } catch {
    return;
  }
}

export type Phase = "login" | "checking" | "wizard" | "already-set-up";

type Wizard = {
  phase: Phase;
  setPhase: (phase: Phase) => void;
  user: CurrentUser | null;
  setUser: (user: CurrentUser | null) => void;
  existingFacility: string;
  setExistingFacility: (name: string) => void;
  progress: Progress;
  update: (patch: Partial<Progress>) => void;
  reset: () => void;
  goTo: (step: StepId) => void;
  complete: (step: StepId, patch?: Partial<Progress>) => void;
  skip: (step: StepId) => void;
};

const WizardContext = createContext<Wizard | null>(null);

export function useWizard(): Wizard {
  const w = useContext(WizardContext);
  if (!w) throw new Error("useWizard must be used inside <WizardProvider>");
  return w;
}

export function nextStep(step: StepId): StepId {
  const i = STEPS.findIndex((s) => s.id === step);
  return i < 0 || i === STEPS.length - 1 ? "done" : STEPS[i + 1].id;
}

export function WizardProvider({ children }: { children: ReactNode }) {
  const [phase, setPhase] = useState<Phase>("login");
  const [user, setUser] = useState<CurrentUser | null>(null);
  const [existingFacility, setExistingFacility] = useState("");
  const [progress, setProgress] = useState<Progress>(loadProgress);

  const commit = useCallback((next: Progress) => {
    saveProgress(next);
    setProgress(next);
  }, []);

  const update = useCallback(
    (patch: Partial<Progress>) => setProgress((p) => {
      const next = { ...p, ...patch };
      saveProgress(next);
      return next;
    }),
    [],
  );

  const reset = useCallback(() => commit(EMPTY_PROGRESS), [commit]);
  const goTo = useCallback((step: StepId) => update({ step }), [update]);

  const complete = useCallback(
    (step: StepId, patch: Partial<Progress> = {}) =>
      setProgress((p) => {
        const next: Progress = {
          ...p,
          ...patch,
          done: { ...p.done, [step]: true },
          skipped: { ...p.skipped, [step]: false },
          step: nextStep(step),
        };
        saveProgress(next);
        return next;
      }),
    [],
  );

  const skip = useCallback(
    (step: StepId) =>
      setProgress((p) => {
        const next: Progress = {
          ...p,
          skipped: { ...p.skipped, [step]: true },
          step: nextStep(step),
        };
        saveProgress(next);
        return next;
      }),
    [],
  );

  const value = useMemo<Wizard>(
    () => ({
      phase,
      setPhase,
      user,
      setUser,
      existingFacility,
      setExistingFacility,
      progress,
      update,
      reset,
      goTo,
      complete,
      skip,
    }),
    [phase, user, existingFacility, progress, update, reset, goTo, complete, skip],
  );

  return <WizardContext.Provider value={value}>{children}</WizardContext.Provider>;
}
