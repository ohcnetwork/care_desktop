import { ExternalLink } from "lucide-react";

import { Screen, ScreenBody, ScreenHead } from "@/components/screen";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardDescription, CardTitle } from "@/components/ui/card";
import { plural } from "@/lib/format";
import { STEPS, useWizard } from "@/state/wizard";

export function DoneScreen() {
  const { progress } = useWizard();
  const skipped = STEPS.filter((s) => progress.skipped[s.id]);
  return (
    <Screen>
      <ScreenHead title="Setup complete" subtitle="CARE is ready for staff to sign in." />
      <ScreenBody>
        <div className="flex max-w-[560px] flex-col gap-4">
          <Card className="px-4 py-3.5">
            <CardTitle>{progress.facilityName}</CardTitle>
            <CardDescription>
              {progress.districtName}, {progress.stateName}
              {progress.departments.length ? ` · ${plural(progress.departments.length, "department")}` : ""}
              {progress.users.length ? ` · ${plural(progress.users.length, "staff account")}` : ""}
            </CardDescription>
          </Card>
          {skipped.length ? (
            <Alert>
              Skipped: {skipped.map((s) => s.label).join(", ")}. These can all be set up from inside CARE
              later.
            </Alert>
          ) : null}
          <Alert>
            This page will not run again while the facility exists. To start over, uninstall CARE Desktop
            and install it again.
          </Alert>
          <div>
            <Button variant="primary" size="lg" asChild>
              <a href="/">
                <ExternalLink className="size-[17px]" strokeWidth={2.2} />
                Open CARE
              </a>
            </Button>
          </div>
        </div>
      </ScreenBody>
    </Screen>
  );
}
