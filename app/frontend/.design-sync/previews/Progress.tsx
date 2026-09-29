import { Progress } from "care-desktop-ui";

export const Installing = () => (
  <div className="w-[520px] rounded-xl border border-line bg-card p-5 shadow-card">
    <div className="mb-2.5 flex items-baseline justify-between">
      <span className="text-[14.5px] font-semibold text-ink">Downloading CARE</span>
      <span className="font-mono text-[13px] font-semibold text-brand-ink">62%</span>
    </div>
    <Progress value={62} />
  </div>
);

export const Values = () => (
  <div className="flex w-[520px] flex-col gap-4">
    <Progress value={8} />
    <Progress value={45} />
    <Progress value={100} />
  </div>
);
