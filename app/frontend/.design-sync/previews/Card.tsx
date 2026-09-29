import { Button, Card, CardDescription, CardTitle } from "care-desktop-ui";

export const Basic = () => (
  <Card className="flex w-[420px] flex-col gap-3 p-6">
    <div>
      <CardTitle>Update CARE</CardTitle>
      <CardDescription>Version 0.1.5 is ready. The clinic restarts for about a minute.</CardDescription>
    </div>
    <div className="flex gap-2">
      <Button variant="primary">Update now</Button>
      <Button>Later</Button>
    </div>
  </Card>
);

export const StatCard = () => (
  <Card className="flex w-[360px] flex-col p-5">
    <div className="text-[11.5px] font-bold tracking-[0.06em] text-muted-foreground uppercase">Backups</div>
    <div className="mt-[7px] text-base font-bold text-ink">Last backup 2 hours ago</div>
    <div className="mt-1 text-[13px] text-muted-foreground">Daily at 04:30 · 14 kept</div>
    <div className="mt-3.5 flex gap-2">
      <Button>View backups</Button>
      <Button variant="soft">Back up now</Button>
    </div>
  </Card>
);

export const DarkHero = () => (
  <div className="flex w-[420px] flex-col rounded-xl bg-brand-deep p-5 text-brand-bg">
    <div className="text-[11.5px] font-bold tracking-[0.06em] text-brand-line uppercase">Clinic address</div>
    <div className="mt-1.5 font-mono text-[26px] font-bold text-white">care.local</div>
    <div className="mt-1.5 text-[13px] text-brand-pale">Open on any device on the clinic WiFi.</div>
    <div className="mt-3.5 flex flex-wrap gap-2">
      <Button variant="white">Open</Button>
      <Button variant="glass">Copy</Button>
    </div>
  </div>
);
