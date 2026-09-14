// One store for everything that outlives a single screen: which view is up, how
// the install is progressing, and the panel's view of the server. Screens own
// their own form state; this owns the parts the rail and the Wails events touch.
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";

import { toast } from "@/components/ui/sonner";
import { bridge, logToHost, onCareEvent } from "@/lib/bridge";
import { errorText, firstLine } from "@/lib/format";
import { RUN_STEPS, type RunStep } from "@/lib/run-steps";
import type { Backup } from "@/types";

export type Flow = "setup" | "installing" | "failed" | "panel";
export type SetupStep = "checks" | "backup" | "admin" | "install";
export type PanelTab = "overview" | "backups" | "advanced";
export type SystemState = "running" | "partial" | "stopped" | "unknown";

export type InstallParams = {
  host: string;
  adminPassword: string;
  backupPassword: string;
  backupDir: string;
};

export type RunState = {
  steps: RunStep[];
  stepIdx: number;
  pct: number;
  startedAt: number;
  finished: boolean;
  failMessage: string;
};

const IDLE_RUN: RunState = {
  steps: RUN_STEPS,
  stepIdx: 0,
  pct: 0,
  startedAt: 0,
  finished: false,
  failMessage: "",
};

const ACTION_LABELS: Record<string, string> = {
  start: "Starting",
  stop: "Stopping",
  restart: "Restarting",
  "rebuild-frontend": "Rebuilding",
  "rebuild-backend": "Rebuilding",
  "backup-now": "Backing up",
};

// Docker Desktop needs about a minute after a reboot before it can answer, and
// the panel starts the clinic itself on launch. Nothing is wrong until then.
const GRACE_MS = 90_000;

const NO_STEPS_DONE: Record<SetupStep, boolean> = {
  checks: false,
  backup: false,
  admin: false,
  install: false,
};

type CareStore = {
  ready: boolean;
  flow: Flow;
  mdnsName: string;

  /** Which setup section is expanded — the rail highlights the same one. */
  openStep: SetupStep;
  setOpenStep: (step: SetupStep) => void;
  stepsDone: Record<SetupStep, boolean>;
  setStepDone: (step: SetupStep, done: boolean) => void;

  run: RunState;
  startInstall: (params: InstallParams) => void;
  retryInstall: () => Promise<void>;
  restartSetup: () => void;
  openPanel: () => void;

  tab: PanelTab;
  setTab: (tab: PanelTab) => void;
  busy: boolean;
  busyLabel: string;
  system: SystemState;
  /** The clinic is down and nobody asked for that - the panel says so. */
  trouble: boolean;
  version: string;
  backups: Backup[];
  autostart: boolean;
  refresh: () => Promise<void>;
  reloadBackups: () => Promise<void>;
  runAction: (action: string) => Promise<void>;
  setAutostart: (on: boolean) => Promise<void>;
  restore: (backup: Backup, passphrase?: string) => Promise<void>;
  uninstall: (removeImages: boolean, removeBackups: boolean) => Promise<void>;
  log: (line: string) => void;
};

const CareContext = createContext<CareStore | null>(null);

export function useCare(): CareStore {
  const store = useContext(CareContext);
  if (!store) throw new Error("useCare must be used inside <CareProvider>");
  return store;
}

export function CareProvider({ children }: { children: ReactNode }) {
  const [ready, setReady] = useState(false);
  const [flow, setFlowState] = useState<Flow>("setup");
  const [mdnsName, setMdnsName] = useState("care.local");
  const [openStep, setOpenStep] = useState<SetupStep>("checks");
  const [stepsDone, setStepsDone] = useState(NO_STEPS_DONE);
  const [run, setRunState] = useState<RunState>(IDLE_RUN);
  const [tab, setTab] = useState<PanelTab>("overview");
  const [busy, setBusyState] = useState(false);
  const [busyLabel, setBusyLabel] = useState("");
  const [system, setSystem] = useState<SystemState>("unknown");
  const [version, setVersion] = useState("");
  const [backups, setBackups] = useState<Backup[]>([]);
  const [autostart, setAutostartState] = useState(false);
  const [trouble, setTrouble] = useState(false);

  // Refs shadow the state the event handlers and the poll timer read, so they
  // never work from a stale closure and never need to re-subscribe.
  const flowRef = useRef<Flow>("setup");
  const runRef = useRef<RunState>(IDLE_RUN);
  const busyRef = useRef(false);
  // Buffered so the fail screen can show the real build error rather than just
  // "exit status 1". Kept out of React state: the install emits thousands of
  // lines and only a step change needs to repaint.
  const logRef = useRef<string[]>([]);
  const lastErrorRef = useRef("");

  // Three things stand between "health check failed" and alarming the operator.
  // Stopping the clinic is a legitimate thing to do, Docker takes about a minute
  // to come up after a reboot, and a container restarting shouldn't raise an
  // alarm that is still on screen after it recovers.
  const stoppedOnPurposeRef = useRef(false);
  const panelSinceRef = useRef(0);
  const downStreakRef = useRef(0);

  const setFlow = useCallback((next: Flow) => {
    flowRef.current = next;
    setFlowState(next);
  }, []);

  const setRun = useCallback((next: RunState) => {
    runRef.current = next;
    setRunState(next);
  }, []);

  const setBusy = useCallback((next: boolean, label = "") => {
    busyRef.current = next;
    setBusyState(next);
    setBusyLabel(label);
  }, []);

  const setStepDone = useCallback((step: SetupStep, done: boolean) => {
    setStepsDone((prev) => (prev[step] === done ? prev : { ...prev, [step]: done }));
  }, []);

  const pushLine = useCallback(
    (line: string) => {
      logRef.current.push(line);
      if (logRef.current.length > 300) logRef.current.shift();
      const current = runRef.current;
      for (let i = 0; i < current.steps.length; i++) {
        if (!current.steps[i].re.test(line)) continue;
        if (current.steps[i].pct > current.pct) {
          setRun({ ...current, stepIdx: i, pct: current.steps[i].pct });
        }
        return;
      }
    },
    [setRun],
  );

  // Lines raised HERE (a failed save, a caught render error) have never been near
  // Go, so they reach the log file only if we send them. Lines arriving on
  // care-log are already in it — see logFromHost below.
  const log = useCallback((line: string) => {
    logToHost(line);
    if (flowRef.current === "panel") {
      return;
    }
    pushLine(line);
  }, [pushLine]);

  // Same buffer and progress-bar handling as log(), minus the write back to the
  // host: Go already put this line in the file.
  const logFromHost = useCallback(
    (line: string) => {
      if (flowRef.current === "panel") return;
      pushLine(line);
    },
    [pushLine],
  );

  const failInstall = useCallback(() => {
    const tail = logRef.current.slice(-40).join("\n").trim();
    const headline = (lastErrorRef.current || "Setup did not complete.").trim();
    setRun({
      ...runRef.current,
      failMessage: tail ? `${headline}\n\n---- last output ----\n${tail}` : headline,
    });
    setFlow("failed");
  }, [setFlow, setRun]);

  // --- panel ------------------------------------------------------------
  const refresh = useCallback(async () => {
    if (busyRef.current || flowRef.current !== "panel") return;
    let next: SystemState;
    try {
      const health = await bridge.ClinicHealth();
      if (health.active) next = "running";
      else {
        let ps = "";
        try {
          ps = await bridge.ClinicStatus();
        } catch {
          ps = "";
        }
        next = ps.trim() ? "partial" : "stopped";
      }
    } catch {
      next = "stopped";
    }
    setSystem(next);

    downStreakRef.current = next === "running" ? 0 : downStreakRef.current + 1;
    const settled = Date.now() - panelSinceRef.current > GRACE_MS;
    setTrouble(
      downStreakRef.current >= 2 && settled && !stoppedOnPurposeRef.current && !busyRef.current,
    );
  }, []);

  const reloadBackups = useCallback(async () => {
    try {
      setBackups(await bridge.ListBackups());
    } catch {
      setBackups([]);
    }
  }, []);

  const runAction = useCallback(
    async (action: string) => {
      if (busyRef.current) return;
      // What the operator asked for, which is what makes a stopped clinic either
      // a fault or a choice. Not persisted: the panel starts the clinic on every
      // launch, so the intent dies with the session, same as the state it describes.
      if (action === "stop") stoppedOnPurposeRef.current = true;
      if (action === "start" || action === "restart") {
        stoppedOnPurposeRef.current = false;
        // Down and being fixed is not down and unattended. refresh() skips while
        // busy, so without this the banner would sit there through the restart.
        downStreakRef.current = 0;
        setTrouble(false);
      }
      setBusy(true, ACTION_LABELS[action] ?? "Working");
      log(`\n$ care ${action}`);
      try {
        await bridge.ClinicAction(action);
      } catch (e) {
        log(`error: ${errorText(e)}`);
        setBusy(false);
      }
    },
    [log, setBusy],
  );

  const syncAutostart = useCallback(async () => {
    try {
      setAutostartState(await bridge.AutostartEnabled());
    } catch {
      /* the host decides; leave the last known value alone */
    }
  }, []);

  const setAutostart = useCallback(
    async (on: boolean) => {
      try {
        await bridge.SetAutostart(on);
        toast(on ? "Start at login on" : "Start at login off");
      } catch (e) {
        log(`autostart error: ${errorText(e)}`);
      }
      await syncAutostart();
    },
    [log, syncAutostart],
  );

  const restore = useCallback(
    async (backup: Backup, passphrase = "") => {
      if (busyRef.current) return;
      setBusy(true, "Restoring");
      log(
        `\n$ care restore ${backup.db_dump}${backup.files_archive ? ` ${backup.files_archive}` : ""}`,
      );
      toast("Restore started — data will be replaced");
      try {
        await bridge.RestoreBackup(backup.db_dump, backup.files_archive, passphrase);
      } catch (e) {
        log(`error: ${errorText(e)}`);
        setBusy(false);
      }
    },
    [log, setBusy],
  );

  const uninstall = useCallback(
    async (removeImages: boolean, removeBackups: boolean) => {
      if (busyRef.current) return;
      setBusy(true, "Uninstalling");
      log(
        `\n$ care uninstall${removeImages ? " --images" : ""}${removeBackups ? " --backups" : ""} --yes`,
      );
      try {
        await bridge.RunUninstall(removeImages, removeBackups);
      } catch (e) {
        log(`error: ${errorText(e)}`);
        setBusy(false);
      }
    },
    [log, setBusy],
  );

  const bootPanel = useCallback(async () => {
    panelSinceRef.current = Date.now();
    stoppedOnPurposeRef.current = false;
    downStreakRef.current = 0;
    setTrouble(false);
    try {
      const state = await bridge.GetState();
      setMdnsName(state.mdns_name || "care.local");
    } catch {
      /* keep whatever the setup flow already told us */
    }
    setTab("overview");
    await reloadBackups();
    await refresh();
    await syncAutostart();
    // Bring the clinic back up on any launch, not just an autostart one: the app
    // quitting is what takes care.local off the network, so opening it again is
    // the operator's way of putting the clinic back. Gated on health, never
    // unconditional — restarting a stack that is already serving would drop the
    // clinic for a minute in the middle of a consultation.
    try {
      const health = await bridge.ClinicHealth();
      if (!health.active && !busyRef.current) {
        log(
          (await bridge.WasAutostartLaunched())
            ? "\nLaunched at startup — starting CARE..."
            : "\nCARE isn't running — starting it...",
        );
        await runAction("start");
      }
    } catch {
      /* can't tell whether it's up — leave it to the operator */
    }
  }, [log, refresh, reloadBackups, runAction, syncAutostart]);

  const openPanel = useCallback(() => {
    setFlow("panel");
    void bootPanel();
  }, [bootPanel, setFlow]);

  // --- setup flow -------------------------------------------------------
  const startInstall = useCallback(
    (params: InstallParams) => {
      logRef.current = [];
      lastErrorRef.current = "";
      setMdnsName(params.host);
      setRun({
        steps: RUN_STEPS,
        stepIdx: 0,
        pct: 0,
        startedAt: Date.now(),
        finished: false,
        failMessage: "",
      });
      setFlow("installing");
      log("Starting one-time setup...");
      void bridge
        .RunSetup(
          params.host,
          params.adminPassword,
          params.backupPassword,
          params.backupDir,
        )
        .catch((e) => {
          lastErrorRef.current = errorText(e);
          log(`error: ${errorText(e)}`);
          failInstall();
        });
    },
    [failInstall, log, setFlow, setRun],
  );

  const restartSetup = useCallback(() => {
    lastErrorRef.current = "";
    logRef.current = [];
    setRun(IDLE_RUN);
    setOpenStep("checks");
    setStepsDone(NO_STEPS_DONE);
    setFlow("setup");
  }, [setFlow, setRun]);

  const retryInstall = useCallback(async () => {
    // Windows: wipe the half-staged install files so the retry re-stages clean. No-op elsewhere.
    try {
      await bridge.CleanupFailedInstall();
    } catch (e) {
      console.log(`cleanup: ${errorText(e)}`);
    }
    restartSetup();
  }, [restartSetup]);

  // --- host events ------------------------------------------------------
  useEffect(() => {
    const unsubscribes = [
      onCareEvent("care-log", (line: string) => {
        if (line.startsWith("error:")) {
          lastErrorRef.current = line.slice("error:".length).trim();
        }
        logFromHost(line);
      }),
      onCareEvent("care-done", (code: number) => {
        if (flowRef.current !== "panel") {
          if (code !== 0) {
            log(`\n× Setup failed (exit ${code}).`);
            failInstall();
          }
          return;
        }
        log(`— done (exit ${code}) —`);
        if (code !== 0) {
          toast(
            lastErrorRef.current
              ? firstLine(lastErrorRef.current)
              : "That action didn't complete.",
          );
        }
        setBusy(false);
        void refresh();
        void reloadBackups();
      }),
      onCareEvent("setup-done", () => {
        const current = runRef.current;
        setRun({
          ...current,
          pct: 100,
          stepIdx: current.steps.length - 1,
          finished: true,
        });
        setStepDone("install", true);
      }),
      onCareEvent("uninstalled", () => {
        toast("Uninstalled");
        window.setTimeout(() => window.location.reload(), 1800);
      }),
    ];
    return () => unsubscribes.forEach((off) => off?.());
  }, [failInstall, log, refresh, reloadBackups, setBusy, setRun, setStepDone]);

  // --- boot + status poll ----------------------------------------------
  useEffect(() => {
    void (async () => {
      try {
        const state = await bridge.GetState();
        setVersion(state.version);
        setMdnsName(state.mdns_name || "care.local");
        if (state.setup_done) {
          setFlow("panel");
          await bootPanel();
        }
      } finally {
        setReady(true);
      }
    })();
    // Boot runs once; bootPanel is stable for the life of the provider.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    const id = window.setInterval(() => void refresh(), 5_000);
    return () => window.clearInterval(id);
  }, [refresh]);

  const value = useMemo<CareStore>(
    () => ({
      ready,
      flow,
      mdnsName,
      openStep,
      setOpenStep,
      stepsDone,
      setStepDone,
      run,
      startInstall,
      retryInstall,
      restartSetup,
      openPanel,
      tab,
      setTab,
      busy,
      busyLabel,
      system,
      trouble,
      version,
      backups,
      autostart,
      refresh,
      reloadBackups,
      runAction,
      setAutostart,
      restore,
      uninstall,
      log,
    }),
    [
      ready, flow, mdnsName, openStep, stepsDone, setStepDone,
      run, startInstall, retryInstall, restartSetup, openPanel,
      tab, busy, busyLabel, system, trouble, version, backups, autostart, refresh, reloadBackups,
      runAction, setAutostart, restore, uninstall, log,
    ],
  );

  return <CareContext.Provider value={value}>{children}</CareContext.Provider>;
}
