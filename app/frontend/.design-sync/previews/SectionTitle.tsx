import { Badge, SectionTitle, StepDot } from "care-desktop-ui";

export const WithBadge = () => (
  <div className="flex w-[560px] flex-col gap-3">
    <div className="flex items-center gap-[13px] rounded-xl border border-line bg-card px-[18px] py-4 shadow-card">
      <StepDot done>1</StepDot>
      <SectionTitle title="Computer check" summary="Everything this clinic needs is ready" />
      <Badge variant="ok">All good</Badge>
    </div>
    <div className="flex items-center gap-[13px] rounded-xl border border-line bg-card px-[18px] py-4 shadow-card">
      <SectionTitle title="Plugins" summary="Extra features for CARE" />
      <Badge variant="plain">3 installed</Badge>
    </div>
    <div className="flex items-center gap-[13px] rounded-xl border border-line bg-card px-[18px] py-4 shadow-card">
      <SectionTitle title="Technical details" />
    </div>
  </div>
);
