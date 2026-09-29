import { Button, Card, StorageRow } from "care-desktop-ui";
import { Database, HardDrive, Server } from "lucide-react";

const GB = 2 ** 30;

export const StorageCard = () => (
  <Card className="w-[640px] overflow-hidden">
    <div className="px-5 pt-4 pb-3">
      <div className="text-[11.5px] font-bold tracking-[0.06em] text-muted-foreground uppercase">Storage</div>
      <div className="mt-0.5 text-[12.5px] text-faint">Checked at 10:42 · every 5 minutes</div>
    </div>
    <StorageRow
      className="border-t border-hair"
      icon={HardDrive}
      label="Macintosh HD"
      path="/"
      free={182 * GB}
      total={494 * GB}
      level="ok"
      message="Plenty of room for the clinic's data."
    />
    <StorageRow
      className="border-t border-hair"
      icon={Server}
      label="Clinic virtual disk"
      free={9 * GB}
      total={60 * GB}
      level="low"
      message="Less than 10 GB left."
      note="The virtual disk Rancher Desktop keeps the clinic's database and uploads in."
      action={<Button size="sm">Free up space</Button>}
    />
    <StorageRow
      className="border-t border-hair"
      icon={Database}
      label="Backups"
      path="/Volumes/CLINIC-USB/CARE Backups"
      free={0.4 * GB}
      total={32 * GB}
      level="critical"
      message="The backup drive is full. New backups will fail."
      action={<Button size="sm">Change folder</Button>}
    />
  </Card>
);
