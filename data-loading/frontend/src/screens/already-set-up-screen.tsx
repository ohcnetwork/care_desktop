import { ExternalLink } from "lucide-react";

import { Screen, ScreenBody, ScreenHead } from "@/components/screen";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardDescription, CardTitle } from "@/components/ui/card";
import { useWizard } from "@/state/wizard";

export function AlreadySetUpScreen() {
  const { existingFacility } = useWizard();
  return (
    <Screen>
      <ScreenHead
        title="This instance is already set up"
        subtitle="A facility exists in CARE, so the first-time setup has already been done here."
      />
      <ScreenBody>
        <div className="flex max-w-[560px] flex-col gap-4">
          <Card className="px-4 py-3.5">
            <CardTitle>Facility</CardTitle>
            <CardDescription>{existingFacility}</CardDescription>
          </Card>
          <Alert>
            Add departments, users, and everything else from inside CARE. Sign in with the
            administrator login and open the facility settings.
          </Alert>
          <Alert>
            To run this setup again from scratch, uninstall CARE Desktop and install it again. That
            removes all data on this instance.
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
