import { RadioChip, RadioGroup } from "care-desktop-ui";

export const ShortList = () => (
  <RadioGroup defaultValue="12h" aria-label="Session timeout">
    <RadioChip value="1h">1 hour</RadioChip>
    <RadioChip value="12h">12 hours</RadioChip>
    <RadioChip value="24h">24 hours</RadioChip>
    <RadioChip value="off" disabled>
      Never
    </RadioChip>
  </RadioGroup>
);
