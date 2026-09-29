import { Badge, Button, Card, CardDescription, CardTitle, FootNote, Screen, ScreenBody, ScreenFoot, ScreenHead } from "care-desktop-ui";

export const SetupPage = () => (
  <div className="flex h-[520px] w-[900px] border border-line bg-background">
    <Screen>
      <ScreenHead
        kicker="Step 2 of 4"
        title="Set up your clinic"
        subtitle="One time, on this computer. About 15 minutes."
        onBack={() => {}}
      />
      <ScreenBody className="flex flex-col gap-3">
        <Card className="flex items-center gap-3 p-5">
          <div className="min-w-0 flex-1">
            <CardTitle>Computer check</CardTitle>
            <CardDescription>Everything this clinic needs is ready</CardDescription>
          </div>
          <Badge variant="ok">All good</Badge>
        </Card>
        <Card className="flex items-center gap-3 p-5">
          <div className="min-w-0 flex-1">
            <CardTitle>Backup</CardTitle>
            <CardDescription>Drive and password for daily backups</CardDescription>
          </div>
          <Badge>To do</Badge>
        </Card>
      </ScreenBody>
      <ScreenFoot>
        <Button variant="primary" size="lg">
          Install and start
        </Button>
        <FootNote>Set and confirm the backup password to continue.</FootNote>
      </ScreenFoot>
    </Screen>
  </div>
);
