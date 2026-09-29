import { Input } from "care-desktop-ui";

export const States = () => (
  <div className="flex w-[360px] flex-col gap-3">
    <Input placeholder="Clinic name" />
    <Input defaultValue="Sunrise Family Clinic" />
    <Input defaultValue="care clinic" aria-invalid />
    <Input defaultValue="care.local" disabled />
  </div>
);

export const Password = () => (
  <div className="w-[360px]">
    <Input type="password" defaultValue="correct-horse-battery" />
  </div>
);
