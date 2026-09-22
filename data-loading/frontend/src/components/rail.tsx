import logoMark from "@/assets/care-logo-mark.svg";
import { cn } from "@/lib/utils";
import { STEPS, useWizard } from "@/state/wizard";

export function Rail() {
  const { phase, progress, user } = useWizard();
  const inWizard = phase === "wizard";

  return (
    <aside className="flex w-[300px] flex-none flex-col bg-brand-deep px-6 py-[26px] text-brand-bg">
      <div className="flex items-center gap-[13px]">
        <img src={logoMark} alt="CARE" className="block h-[52px] w-auto brightness-0 invert" />
        <div>
          <div className="text-base leading-tight font-bold text-white">CARE</div>
          <div className="text-[12.5px] text-brand-pale">Facility setup</div>
        </div>
      </div>

      <div className="mt-6 mb-5 h-px bg-white/[0.18]" />

      {inWizard ? (
        <div className="flex flex-col gap-0.5">
          {STEPS.map((step, i) => {
            const done = !!progress.done[step.id];
            const skipped = !!progress.skipped[step.id];
            const active = progress.step === step.id;
            return (
              <div
                key={step.id}
                className={cn(
                  "flex items-center gap-3 rounded-lg px-2.5 py-[10px] text-[13.5px] text-[#d3f5e5]",
                  active && "bg-white/[0.14]",
                  (done || skipped) && "text-white",
                )}
              >
                <span
                  className={cn(
                    "flex size-[22px] flex-none items-center justify-center rounded-full text-xs font-bold",
                    done ? "bg-white text-brand-ink" : "bg-white/[0.18] text-[#e3fbf0]",
                  )}
                >
                  {done ? "✓" : skipped ? "–" : i + 1}
                </span>
                <span>{step.label}</span>
              </div>
            );
          })}
        </div>
      ) : null}

      <div className="flex-1" />

      {user ? (
        <div className="rounded-[11px] bg-white/10 px-[13px] py-3 text-[13px] text-white">
          <div className="text-[11px] font-semibold tracking-[0.04em] text-brand-pale uppercase">
            Signed in as
          </div>
          <div className="font-mono font-semibold">{user.username}</div>
        </div>
      ) : null}
    </aside>
  );
}
