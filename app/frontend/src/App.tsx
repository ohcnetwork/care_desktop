import { useCallback, useEffect, useRef, useState } from "react";

import { Rail } from "@/components/rail";
import { normaliseHost } from "@/lib/format";
import { ClinicScreen } from "@/screens/clinic/clinic-screen";
import { FailedScreen } from "@/screens/install/failed-screen";
import { InstallingScreen } from "@/screens/install/installing-screen";
import { PanelScreen } from "@/screens/panel/panel-screen";
import { SetupScreen } from "@/screens/setup/setup-screen";
import { useCare, type InstallParams } from "@/state/care-store";
import {
  EMPTY_CLINIC_FORM,
  EMPTY_SETUP_FORM,
  type ClinicForm,
  type SetupForm,
} from "@/state/forms";

export function App() {
  const care = useCare();
  // Both forms live here rather than in their screens, so stepping forward to
  // the clinic details and back — or bouncing off a failed install — keeps
  // everything the operator already typed.
  const [setupForm, setSetupForm] = useState<SetupForm>(EMPTY_SETUP_FORM);
  const [clinicForm, setClinicForm] = useState<ClinicForm>(EMPTY_CLINIC_FORM);
  const seeded = useRef(false);

  // The host already has a name for this clinic; adopt it as the field's value.
  useEffect(() => {
    if (!care.ready || seeded.current) return;
    seeded.current = true;
    setSetupForm((form) => ({
      ...form,
      hostInput: care.mdnsName.replace(/\.local$/i, ""),
    }));
  }, [care.ready, care.mdnsName]);

  const patchSetup = useCallback(
    (values: Partial<SetupForm>) => setSetupForm((form) => ({ ...form, ...values })),
    [],
  );
  const patchClinic = useCallback(
    (values: Partial<ClinicForm>) => setClinicForm((form) => ({ ...form, ...values })),
    [],
  );
  const installParams = useCallback(
    (): InstallParams => ({
      host: normaliseHost(setupForm.hostInput),
      adminPassword: setupForm.adminPassword,
      backupPassword: setupForm.backupPassword,
      backupDir: setupForm.backupDir,
    }),
    [setupForm],
  );

  return (
    <div className="flex h-full">
      <Rail />
      {care.ready ? (
        care.flow === "setup" ? (
          <SetupScreen form={setupForm} patch={patchSetup} />
        ) : care.flow === "clinic" ? (
          <ClinicScreen
            form={clinicForm}
            patch={patchClinic}
            installParams={installParams}
          />
        ) : care.flow === "installing" ? (
          <InstallingScreen />
        ) : care.flow === "failed" ? (
          <FailedScreen />
        ) : (
          <PanelScreen />
        )
      ) : (
        <div className="min-w-0 flex-1 bg-background" />
      )}
    </div>
  );
}
