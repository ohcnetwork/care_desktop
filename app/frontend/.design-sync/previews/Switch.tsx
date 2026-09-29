import { Switch } from "care-desktop-ui";

export const WithLabel = () => (
  <div className="flex flex-col gap-3">
    <label className="flex cursor-pointer items-center gap-[9px] text-[13px] font-semibold text-ink2 select-none">
      <Switch defaultChecked aria-label="Start at login" />
      <span>Start at login</span>
    </label>
    <label className="flex cursor-pointer items-center gap-[9px] text-[13px] font-semibold text-ink2 select-none">
      <Switch aria-label="Encrypt backups" />
      <span>Encrypt backups</span>
    </label>
    <label className="flex items-center gap-[9px] text-[13px] font-semibold text-faint select-none">
      <Switch disabled aria-label="Remote access" />
      <span>Remote access</span>
    </label>
  </div>
);
