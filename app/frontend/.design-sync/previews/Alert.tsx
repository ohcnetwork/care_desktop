import { Alert } from "care-desktop-ui";
import { CircleAlert, TriangleAlert } from "lucide-react";

export const Variants = () => (
  <div className="flex w-[520px] flex-col gap-3">
    <Alert>
      Daily backups are encrypted with this password. If it is lost, the backups cannot be opened by anyone,
      including us. Store it somewhere safe.
    </Alert>
    <Alert variant="warn">
      <TriangleAlert className="size-4 flex-none" />
      The backup drive is getting full. Free up space or choose another folder soon.
    </Alert>
    <Alert variant="danger">
      <CircleAlert className="size-4 flex-none" />
      Removing CARE deletes every patient record on this computer. This cannot be undone.
    </Alert>
  </div>
);
