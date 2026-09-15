import { Screen } from "@/components/screen";
import { SectionTitle } from "@/components/section-header";
import { Spinner } from "@/components/spinner";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { useElapsed } from "@/hooks/use-elapsed";
import { mmss } from "@/lib/format";
import { cn } from "@/lib/utils";
import { useCare } from "@/state/care-store";

export function InstallingScreen() {
  const { run, mdnsName, openPanel } = useCare();
  const elapsed = useElapsed(run.startedAt, !run.finished);
  const done = run.finished || run.pct >= 100;

  return (
    <Screen className="px-[42px] pt-[44px] pb-[30px]">
      <div className="flex items-start gap-3.5">
        <span
          className={cn(
            "flex size-[34px] flex-none items-center justify-center rounded-full text-base font-bold",
            done ? "bg-brand text-white" : "bg-brand-bg text-brand-ink",
          )}
        >
          {done ? "✓" : <Spinner />}
        </span>
        <div className="min-w-0 flex-1">
          <h1 className="text-2xl font-bold tracking-[-0.015em] text-ink">
            {done ? "Your clinic is ready" : "Setting up your clinic"}
          </h1>
          <p className="mt-1.5 text-sm text-muted-foreground">
            {done
              ? "CARE is running on this computer and reachable on the clinic WiFi."
              : "This takes about 10 to 20 minutes."}
          </p>
        </div>
        <span className="font-mono text-faint">{mmss(elapsed)}</span>
      </div>

      <div className="min-h-0 flex-1 overflow-auto">
        <div className="mt-[26px]">
          <div className="mb-[9px] flex items-baseline justify-between text-[15px] font-semibold text-ink">
            <span>{done ? "Done" : run.steps[run.stepIdx].label}</span>
            <span className="font-mono text-[22px] font-bold text-brand-ink">
              {Math.floor(run.pct)}%
            </span>
          </div>
          <Progress value={run.pct} />
        </div>

        <Accordion type="single" collapsible className="mt-5">
          <AccordionItem value="details">
            <AccordionTrigger>
              <SectionTitle
                title="Details"
                summary={
                  done
                    ? `All ${run.steps.length} steps finished`
                    : `Step ${run.stepIdx + 1} of ${run.steps.length}`
                }
              />
            </AccordionTrigger>
            <AccordionContent className="gap-0">
              {run.steps.map((step, i) => {
                const state = done || i < run.stepIdx ? "done" : i === run.stepIdx ? "now" : "todo";
                return (
                  <div
                    key={step.label}
                    className={cn(
                      "flex items-center gap-[11px] py-[9px]",
                      i > 0 && "border-t border-hair",
                    )}
                  >
                    <span
                      className={cn(
                        "flex size-[18px] flex-none items-center justify-center rounded-full text-[11px] font-bold",
                        state === "done"
                          ? "bg-brand text-white"
                          : state === "now"
                            ? "bg-brand-bg text-brand-ink"
                            : "bg-hair text-[#d1d5db]",
                      )}
                    >
                      {state === "done" ? "✓" : state === "now" ? <Spinner className="size-[11px]" /> : "·"}
                    </span>
                    <span
                      className={cn(
                        "flex-1 text-[13.5px] font-medium",
                        state === "todo" ? "text-faint" : "text-ink",
                        state === "now" && "font-semibold",
                      )}
                    >
                      {step.label}
                    </span>
                    <span className="font-mono text-xs text-faint">
                      {state === "done" ? "done" : state === "now" ? "working" : ""}
                    </span>
                  </div>
                );
              })}
            </AccordionContent>
          </AccordionItem>
        </Accordion>
      </div>

      {run.finished ? (
        <div className="flex items-center gap-3.5">
          <Button
            variant="primary"
            size="lg"
            className="shadow-lift"
            onClick={openPanel}
          >
            Open control panel
          </Button>
          <span className="text-[13px] text-muted-foreground">
            Clinic address <b className="font-mono text-brand-ink">{mdnsName}</b>
          </span>
        </div>
      ) : null}
    </Screen>
  );
}
