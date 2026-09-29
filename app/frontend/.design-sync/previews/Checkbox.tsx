import { Checkbox } from "care-desktop-ui";

export const DestructiveConfirm = () => (
  <div className="flex w-[440px] flex-col gap-3">
    <label className="flex cursor-pointer items-start gap-2.5 text-[13px] text-ink2">
      <Checkbox defaultChecked className="mt-0.5" />
      <span>Also delete all patient records and uploaded files</span>
    </label>
    <label className="flex cursor-pointer items-start gap-2.5 text-[13px] text-ink2">
      <Checkbox className="mt-0.5" />
      <span>Also delete the backup folder</span>
    </label>
    <label className="flex cursor-not-allowed items-start gap-2.5 text-[13px] text-faint">
      <Checkbox disabled className="mt-0.5" />
      <span>Remove Rancher Desktop (not installed by CARE)</span>
    </label>
  </div>
);
