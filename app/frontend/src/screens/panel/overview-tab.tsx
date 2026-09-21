import { Spinner } from "@/components/spinner";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { toast } from "@/components/ui/sonner";
import { Switch } from "@/components/ui/switch";
import { bridge } from "@/lib/bridge";
import { shortDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import { RESTORE_PENDING_NOTICE, useCare, type SystemState } from "@/state/care-store";

const SYSTEM: Record<
  Exclude<SystemState, "unknown">,
  { label: string; sub: string; glyph: string; tone: string }
> = {
  running: {
    label: "Running",
    sub: "Live and reachable on the clinic WiFi.",
    glyph: "●",
    tone: "bg-brand-bg text-brand-ink",
  },
  stopped: {
    label: "Stopped",
    sub: "The clinic system is not running.",
    glyph: "■",
    tone: "bg-hair text-muted-foreground",
  },
  partial: {
    label: "Starting…",
    sub: "Some services are still coming up.",
    glyph: "■",
    tone: "bg-warn-bg text-warn-ink",
  },
};

export function OverviewTab() {
  const {
    system,
    systemDetail,
    restorePending,
    busy,
    busyLabel,
    autostart,
    setAutostart,
    runAction,
    backups,
    mdnsName,
    setTab,
  } = useCare();

  const view = SYSTEM[system === "unknown" ? "stopped" : system];
  const running = system === "running";
  const partial = system === "partial";
  const stopped = !running && !partial;
  const unreachable = system === "unknown" && systemDetail !== "";
  const latest = backups[0];

  const copyAddress = () =>
    void navigator.clipboard.writeText(mdnsName).then(
      () => toast("Address copied"),
      () => toast(mdnsName),
    );

  return (
    <div className="flex flex-col gap-3">
      {restorePending ? (
        <Alert variant="danger">
          {RESTORE_PENDING_NOTICE} Recovery data is kept until CARE starts successfully.
        </Alert>
      ) : null}
      <Card className="flex items-center gap-4 p-5">
        <span
          className={cn(
            "flex size-10 flex-none items-center justify-center rounded-full text-base font-bold",
            busy ? "bg-warn-bg text-warn-ink" : unreachable ? "bg-danger-bg text-danger-ink" : view.tone,
          )}
        >
          {busy ? <Spinner /> : unreachable ? "!" : view.glyph}
        </span>
        <div className="min-w-0 flex-1">
          <div className="text-[17px] font-bold text-ink">
            {busy
              ? `${busyLabel}…`
              : system === "unknown"
                ? unreachable
                  ? "Can't check the clinic"
                  : "checking…"
                : view.label}
          </div>
          <div className="mt-0.5 text-[13.5px] text-muted-foreground">
            {busy ? "Please wait a moment." : system === "unknown" ? systemDetail : view.sub}
          </div>
        </div>
        <div className="flex flex-none items-center gap-2">
          <Button
            variant={stopped && !busy ? "primary" : "default"}
            disabled={busy || running || partial}
            onClick={() => void runAction("start")}
          >
            Start
          </Button>
          <Button disabled={busy || stopped} onClick={() => void runAction("stop")}>
            Stop
          </Button>
          <Button
            disabled={busy || stopped || restorePending}
            onClick={() => void runAction("restart")}
          >
            Restart
          </Button>
          <label className="ml-1.5 flex cursor-pointer items-center gap-[9px] text-[13px] font-semibold text-ink2 select-none">
            <Switch
              checked={autostart}
              onCheckedChange={(on) => void setAutostart(on)}
              aria-label="Start at login"
            />
            <span>Start at login</span>
          </label>
        </div>
      </Card>

      <div className="grid grid-cols-[1.15fr_1fr] gap-3">
        <div className="flex flex-col rounded-xl bg-brand-deep p-5 text-brand-bg">
          <div className="text-[11.5px] font-bold tracking-[0.06em] text-brand-line uppercase">
            Clinic address
          </div>
          <div className="mt-1.5 font-mono text-[26px] font-bold text-white">{mdnsName}</div>
          <div className="mt-1.5 text-[13px] text-brand-pale">
            Open on any device on the clinic WiFi.
          </div>
          <div className="min-h-3.5 flex-1" />
          <div className="flex gap-2">
            <Button variant="white" onClick={() => void bridge.OpenURL(`https://${mdnsName}/`)}>
              Open
            </Button>
            <Button variant="glass" onClick={copyAddress}>
              Copy
            </Button>
          </div>
        </div>

        <Card className="flex flex-col p-5">
          <div className="text-[11.5px] font-bold tracking-[0.06em] text-muted-foreground uppercase">
            Backups
          </div>
          <div className="mt-[7px] text-base font-bold text-ink">
            {latest ? "Up to date" : "No backups yet"}
          </div>
          <div className="mt-1 text-[13px] text-muted-foreground">
            {latest
              ? `Last ${shortDate(latest.label)}${latest.encrypted ? ", encrypted" : ""}`
              : "Run one now or wait for the daily backup"}
          </div>
          <div className="min-h-3.5 flex-1" />
          <div className="flex gap-2">
            <Button onClick={() => setTab("backups")}>View backups</Button>
            <Button
              variant="soft"
              disabled={busy || restorePending}
              onClick={() => void runAction("backup-now")}
            >
              Back up now
            </Button>
          </div>
        </Card>
      </div>
    </div>
  );
}
