import { useCallback, useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { bridge } from "@/lib/bridge";
import { errorText, firstLine } from "@/lib/format";
import { useCare } from "@/state/care-store";
import type { AppUpdate, ChannelStatus } from "@/types";

const short = (sha: string) => (sha ? sha.slice(0, 8) : "");

export function UpdatePanel() {
  return (
    <div className="flex flex-col gap-3">
      <CareChannelCard />
      <AppUpdateCard />
    </div>
  );
}

function CareChannelCard() {
  const { log, busy, careUpdate, applyCareUpdate } = useCare();
  const [status, setStatus] = useState<ChannelStatus | null>(null);
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState("");

  const reload = useCallback(async () => {
    try {
      setStatus(await bridge.CareUpdateStatus());
      setError("");
    } catch (e) {
      setError(firstLine(errorText(e)));
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload, careUpdate, busy]);

  const check = async () => {
    setChecking(true);
    setError("");
    try {
      await bridge.CheckCareUpdate();
      log("Checking for CARE updates in the background...");
    } catch (e) {
      setError(firstLine(errorText(e)));
    } finally {
      setChecking(false);
    }
  };

  const pending = status?.pending_backend || status?.pending_frontend || "";

  return (
    <div className="rounded-xl border border-line bg-card px-[18px] py-4 shadow-card">
      <div className="flex items-center gap-3.5">
        <div className="min-w-0 flex-1">
          <div className="text-[15px] font-bold text-ink">CARE</div>
          <div className="mt-[3px] text-[13px] text-muted-foreground">
            Follows the{" "}
            <span className="font-mono text-ink2">{status?.backend_branch || "…"}</span> branch and
            updates itself. New versions are downloaded and built in the background.
          </div>
        </div>
        <Button disabled={checking || busy} onClick={() => void check()}>
          {checking ? "Checking…" : "Check now"}
        </Button>
      </div>

      <dl className="mt-3.5 grid grid-cols-2 gap-x-4 gap-y-2 border-t border-hair pt-3.5 text-[13px]">
        <Row label="Backend" value={short(status?.backend ?? "")} />
        <Row label="Frontend" value={short(status?.frontend ?? "")} />
      </dl>

      {pending ? (
        <div className="mt-3.5 flex items-center gap-3 rounded-lg border border-line bg-brand-bg px-4 py-[13px] text-[12.5px] text-brand-ink">
          <span className="flex-1">
            An update is built and ready. It installs on the next start, or now.
          </span>
          <Button variant="primary" disabled={busy} onClick={() => void applyCareUpdate()}>
            Install now
          </Button>
        </div>
      ) : null}

      {error ? <div className="mt-2.5 text-[12.5px] text-danger-ink">{error}</div> : null}
    </div>
  );
}

function AppUpdateCard() {
  const { busy } = useCare();
  const [update, setUpdate] = useState<AppUpdate | null>(null);
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState("");

  const check = useCallback(async (announce: boolean) => {
    setChecking(true);
    setError("");
    try {
      setUpdate(await bridge.CheckAppUpdate());
    } catch (e) {
      if (announce) setError(firstLine(errorText(e)));
    } finally {
      setChecking(false);
    }
  }, []);

  useEffect(() => {
    void check(false);
  }, [check]);

  return (
    <div className="rounded-xl border border-line bg-card px-[18px] py-4 shadow-card">
      <div className="flex items-center gap-3.5">
        <div className="min-w-0 flex-1">
          <div className="text-[15px] font-bold text-ink">CARE Desktop</div>
          <div className="mt-[3px] text-[13px] text-muted-foreground">
            This application. Version{" "}
            <span className="font-mono text-ink2">{update?.current || "…"}</span>
            {update?.available ? (
              <>
                {" — "}
                <span className="font-semibold text-brand-ink">
                  {update.version} is available
                </span>
              </>
            ) : update?.version ? (
              " — up to date"
            ) : null}
          </div>
        </div>
        <Button disabled={checking || busy} onClick={() => void check(true)}>
          {checking ? "Checking…" : "Check now"}
        </Button>
      </div>

      {update?.available ? (
        <div className="mt-3.5 flex items-center gap-3 rounded-lg border border-line bg-brand-bg px-4 py-[13px] text-[12.5px] text-brand-ink">
          <span className="flex-1">
            Downloads {update.asset} and checks it against the published checksum.
            Patient data and clinic settings are untouched.
          </span>
          {update.notes_url ? (
            <Button onClick={() => void bridge.OpenURL(update.notes_url)}>Release notes</Button>
          ) : null}
          <Button
            variant="primary"
            disabled={busy}
            onClick={() => void bridge.InstallAppUpdate()}
          >
            Download and install
          </Button>
        </div>
      ) : null}

      {error ? <div className="mt-2.5 text-[12.5px] text-danger-ink">{error}</div> : null}
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline gap-2">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="font-mono font-semibold text-ink">{value || "not built yet"}</dd>
    </div>
  );
}
