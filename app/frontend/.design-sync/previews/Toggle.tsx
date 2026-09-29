import { Toggle } from "care-desktop-ui";
import { Bell } from "lucide-react";

export const States = () => (
  <div className="flex flex-wrap items-center gap-2">
    <Toggle>Email alerts</Toggle>
    <Toggle defaultPressed>SMS alerts</Toggle>
    <Toggle disabled>WhatsApp alerts</Toggle>
  </div>
);

export const SmallWithIcon = () => (
  <div className="flex flex-wrap items-center gap-2">
    <Toggle size="sm" defaultPressed>
      <Bell className="size-3.5" />
      Notify staff
    </Toggle>
    <Toggle size="sm">Quiet hours</Toggle>
  </div>
);
