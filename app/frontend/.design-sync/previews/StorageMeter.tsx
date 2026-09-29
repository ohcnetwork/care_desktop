import { StorageMeter } from "care-desktop-ui";

const GB = 2 ** 30;

export const Levels = () => (
  <div className="flex w-[420px] flex-col gap-3">
    {([
      ["ok", 180 * GB, "Enough room"],
      ["low", 30 * GB, "Getting full"],
      ["critical", 4 * GB, "Full"],
      ["unknown", 0, "Unknown"],
    ] as const).map(([level, free, label]) => (
      <div key={level} className="flex items-center gap-3">
        <span className="w-[100px] flex-none text-[12.5px] text-muted-foreground">{label}</span>
        <StorageMeter free={free} total={level === "unknown" ? 0 : 256 * GB} level={level} />
      </div>
    ))}
  </div>
);
