import { RadioChip, RadioGroup, RadioGroupItem } from "care-desktop-ui";

export const Chips = () => (
  <RadioGroup defaultValue="yes" aria-label="Allow sign-ups">
    <RadioChip value="yes">Yes</RadioChip>
    <RadioChip value="no">No</RadioChip>
  </RadioGroup>
);

export const BareItems = () => (
  <RadioGroup defaultValue="usb" orientation="vertical" className="flex-col gap-3" aria-label="Backup location">
    <label className="flex items-center gap-2.5 text-[13px] text-ink2">
      <RadioGroupItem value="desktop" /> Desktop folder
    </label>
    <label className="flex items-center gap-2.5 text-[13px] text-ink2">
      <RadioGroupItem value="usb" /> USB drive
    </label>
    <label className="flex items-center gap-2.5 text-[13px] text-faint">
      <RadioGroupItem value="cloud" disabled /> Cloud (coming soon)
    </label>
  </RadioGroup>
);
