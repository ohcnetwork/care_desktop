import { BoxNote, Field, Input, InputBox } from "care-desktop-ui";

export const WithExplainer = () => (
  <div className="w-[480px]">
    <Field
      label="Clinic address"
      htmlFor="mdnsname"
      info="The address staff type in their browser. Change it only if another CARE clinic is already running on this WiFi, so the two names do not clash."
      infoTitle="The address staff type in their browser"
      messages={<span>Staff will open https://care.local</span>}
    >
      <InputBox>
        <Input
          id="mdnsname"
          defaultValue="care"
          className="h-full flex-1 rounded-none border-none bg-transparent px-0 focus-visible:border-none"
        />
        <BoxNote>.local</BoxNote>
      </InputBox>
    </Field>
  </div>
);

export const WithError = () => (
  <div className="w-[480px]">
    <Field
      label="Clinic address"
      htmlFor="mdnsname-bad"
      messages={<span className="text-danger-ink">Use only letters, numbers and hyphens.</span>}
    >
      <InputBox tone="bad">
        <Input
          id="mdnsname-bad"
          defaultValue="care clinic"
          className="h-full flex-1 rounded-none border-none bg-transparent px-0 focus-visible:border-none"
        />
        <BoxNote>.local</BoxNote>
      </InputBox>
    </Field>
  </div>
);

export const PlainInput = () => (
  <div className="w-[480px]">
    <Field label="Backup folder" htmlFor="dir" messages="Choose a USB drive to keep backups safe if this computer fails.">
      <Input id="dir" placeholder="/Volumes/Backup" />
    </Field>
  </div>
);
