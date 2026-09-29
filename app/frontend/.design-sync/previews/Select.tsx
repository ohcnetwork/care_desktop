import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from "care-desktop-ui";

export const Open = () => (
  <div className="h-[300px] w-[280px]">
    <Select defaultValue="scribe" defaultOpen>
      <SelectTrigger aria-label="Add a plugin">
        <SelectValue placeholder="Add a plugin" />
      </SelectTrigger>
      <SelectContent>
        <SelectGroup>
          <SelectLabel>Available plugins</SelectLabel>
          <SelectItem value="scribe">Scribe</SelectItem>
          <SelectItem value="abdm">ABDM</SelectItem>
          <SelectItem value="hcx">HCX claims</SelectItem>
          <SelectItem value="teleicu">Tele-ICU</SelectItem>
        </SelectGroup>
      </SelectContent>
    </Select>
  </div>
);

export const Closed = () => (
  <div className="flex w-[280px] flex-col gap-3">
    <Select>
      <SelectTrigger aria-label="Add a plugin">
        <SelectValue placeholder="Add a plugin" />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="scribe">Scribe</SelectItem>
      </SelectContent>
    </Select>
    <Select defaultValue="ist">
      <SelectTrigger aria-label="Time zone">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="ist">Asia/Kolkata (IST)</SelectItem>
      </SelectContent>
    </Select>
  </div>
);
