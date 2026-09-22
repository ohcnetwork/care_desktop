import { Rail } from "@/components/rail";
import { AlreadySetUpScreen } from "@/screens/already-set-up-screen";
import { CheckingScreen } from "@/screens/checking-screen";
import { DoneScreen } from "@/screens/done-screen";
import { LoginScreen } from "@/screens/login-screen";
import { useWizard, type StepId } from "@/state/wizard";
import { ClinicalStep } from "@/steps/clinical-step";
import { ContentStep } from "@/steps/content-step";
import { DepartmentsStep } from "@/steps/departments-step";
import { DistrictStep } from "@/steps/district-step";
import { FacilityStep } from "@/steps/facility-step";
import { InvoiceStep } from "@/steps/invoice-step";
import { PatientIdStep } from "@/steps/patient-id-step";
import { StatesStep } from "@/steps/states-step";
import { UsersStep } from "@/steps/users-step";

const STEP_SCREENS: Record<StepId, () => React.JSX.Element> = {
  states: StatesStep,
  district: DistrictStep,
  facility: FacilityStep,
  departments: DepartmentsStep,
  users: UsersStep,
  clinical: ClinicalStep,
  invoice: InvoiceStep,
  "patient-id": PatientIdStep,
  content: ContentStep,
  done: DoneScreen,
};

export function App() {
  const { phase, progress } = useWizard();
  const Step = STEP_SCREENS[progress.step] ?? StatesStep;
  return (
    <div className="flex h-full">
      <Rail />
      {phase === "login" ? (
        <LoginScreen />
      ) : phase === "checking" ? (
        <CheckingScreen />
      ) : phase === "already-set-up" ? (
        <AlreadySetUpScreen />
      ) : (
        <Step key={progress.step} />
      )}
    </div>
  );
}
