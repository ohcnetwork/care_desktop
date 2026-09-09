import { Fragment, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { Check } from "./use-requirement-checks";

const DOT: Record<Check["state"], string> = {
  wait: "bg-line text-faint",
  ok: "bg-brand text-white",
  bad: "bg-danger text-white",
};

const GLYPH: Record<Check["state"], string> = { wait: "·", ok: "✓", bad: "✗" };

export function CheckRows({
  checks,
  onFixNetwork,
}: {
  checks: Check[];
  onFixNetwork: () => Promise<void>;
}) {
  const [fixing, setFixing] = useState(false);

  return (
    <div className="overflow-hidden rounded-lg border border-line">
      {checks.map((check, i) => (
        <Fragment key={check.id}>
          <div
            className={cn(
              "flex items-center gap-3 px-3.5 py-[13px]",
              i > 0 && "border-t border-line",
              check.state === "bad" && "bg-danger-tint",
            )}
          >
            <span
              className={cn(
                "flex size-5 flex-none items-center justify-center rounded-full text-[11px] font-bold",
                DOT[check.state],
              )}
            >
              {GLYPH[check.state]}
            </span>
            <div className="min-w-0 flex-1">
              <div className="text-[13.5px] font-semibold text-ink">{check.title}</div>
              <div className="mt-px text-[12.5px] text-muted-foreground">{check.detail}</div>
            </div>
            <Badge variant={check.state === "wait" ? "default" : check.state}>
              {check.state === "wait" ? "Checking" : check.state === "ok" ? "Ready" : "Not ready"}
            </Badge>
          </div>
          {check.state === "bad" && check.how ? (
            <div className="flex items-center gap-3 border-t border-danger-bg bg-[#fef6f6] py-3 pr-3.5 pl-[46px]">
              <span className="flex-1 text-[12.5px] leading-[1.5] text-danger-ink">
                {check.how}
              </span>
              {check.fixable ? (
                <Button
                  variant="destructive"
                  disabled={fixing}
                  onClick={() => {
                    setFixing(true);
                    void onFixNetwork().finally(() => setFixing(false));
                  }}
                >
                  {fixing ? "Fixing…" : "Fix automatically"}
                </Button>
              ) : null}
            </div>
          ) : null}
        </Fragment>
      ))}
    </div>
  );
}
