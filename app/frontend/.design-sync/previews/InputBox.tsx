import { BoxNote, Input, InputBox } from "care-desktop-ui";

const BARE = "h-full flex-1 rounded-none border-none bg-transparent px-0 focus-visible:border-none";

export const Tones = () => (
  <div className="flex w-[420px] flex-col gap-3">
    <div className="flex">
      <InputBox>
        <Input defaultValue="care" className={BARE} />
        <BoxNote>.local</BoxNote>
      </InputBox>
    </div>
    <div className="flex">
      <InputBox tone="ok">
        <Input type="password" defaultValue="Clinic-Backup-2026" className={BARE} />
        <BoxNote tone="ok">Match</BoxNote>
      </InputBox>
    </div>
    <div className="flex">
      <InputBox tone="bad">
        <Input type="password" defaultValue="Clinic-Backup" className={BARE} />
        <BoxNote tone="bad">No match</BoxNote>
      </InputBox>
    </div>
  </div>
);

export const PasswordPair = () => (
  <div className="w-[560px]">
    <div className="flex gap-2.5">
      <InputBox tone="ok">
        <Input type="password" placeholder="Password" defaultValue="Clinic-Backup-2026" className={BARE} />
        <button type="button" className="cursor-pointer p-1 text-xs font-semibold text-muted-foreground hover:text-brand-ink">
          Show
        </button>
      </InputBox>
      <InputBox tone="ok">
        <Input type="password" placeholder="Confirm password" defaultValue="Clinic-Backup-2026" className={BARE} />
        <BoxNote tone="ok">Match</BoxNote>
      </InputBox>
    </div>
    <div className="mt-[9px] flex flex-col gap-1 text-[12.5px] leading-[1.5] text-muted-foreground">
      <span className="text-brand-ink">Strong password.</span>
      <span className="text-brand-ink">Both passwords match.</span>
    </div>
  </div>
);
