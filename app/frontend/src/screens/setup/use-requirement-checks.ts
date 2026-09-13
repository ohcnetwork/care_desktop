import { useCallback, useEffect, useMemo, useState } from "react";

import { bridge } from "@/lib/bridge";
import type { ToolPlan } from "@/types";

export type CheckTone = "wait" | "ok" | "bad";
export type CheckId = "residue" | "docker" | "git" | "mdns" | "network";

/**
 * What the operator can press on a failing row. The wizard is used by people who
 * will not open a terminal, so a check that can't be acted on is a dead end -
 * every failure that we know how to fix carries the fix.
 *
 * run resolves to a confirmation to show when the work succeeded, or nothing
 * when there is none worth a dialog (opening Docker, opening a download page).
 */
export type CheckAction = {
  label: string;
  detail: string;
  run: () => Promise<string | void>;
};

export type Check = {
  id: CheckId;
  title: string;
  detail: string;
  state: CheckTone;
  how: string;
  action?: CheckAction;
};

type Result = { state: CheckTone; how: string; action?: CheckAction };

const WAITING: Result = { state: "wait", how: "" };

function summarise(results: Result[]): CheckTone {
  if (results.some((r) => r.state === "bad")) return "bad";
  return results.some((r) => r.state === "wait") ? "wait" : "ok";
}

/**
 * Turns a plan from the host into a button. "manual" means we can't do it here,
 * so the button opens the vendor's download page instead of pretending.
 */
function actionFor(plan: ToolPlan, install: () => Promise<string | void>): CheckAction | undefined {
  if (plan.action === "" || plan.label === "") return undefined;
  if (plan.action === "manual") {
    return {
      label: plan.label,
      detail: plan.detail,
      run: () => bridge.OpenURL(plan.url),
    };
  }
  return { label: plan.label, detail: plan.detail, run: install };
}

/**
 * The prerequisites the app can't bundle, one row each. Docker and git are
 * separate rows rather than one "runtime" row because they are fixed in
 * different ways, and a row can only carry one button.
 */
export function useRequirementChecks(host: string) {
  const [residue, setResidue] = useState<Result>(WAITING);
  const [docker, setDocker] = useState<Result>(WAITING);
  const [git, setGit] = useState<Result>(WAITING);
  const [mdns, setMdns] = useState<Result>(WAITING);
  const [network, setNetwork] = useState<Result | null>(null);

  /**
   * Leftovers from an earlier CARE Desktop. This is a hard blocker rather than a
   * warning: an old data volume still carrying the label gets re-attached by
   * compose, so the "new" clinic comes up holding the previous one's patients.
   *
   * Docker-side traces are invisible while Docker is down, so this is
   * deliberately re-run as part of recheckAll - once Docker goes green the scan
   * sees the containers, volumes, and images it could not see before.
   */
  const checkResidue = useCallback(async (): Promise<Result> => {
    setResidue(WAITING);
    let result: Result;
    try {
      const report = await bridge.ScanResidue();
      result = report.clean
        ? { state: "ok", how: "" }
        : {
            state: "bad",
            how: `Found ${report.traces.map((t) => t.label.toLowerCase()).join(", ")}. These must be removed before a new clinic can be set up.`,
            action: {
              label: "Remove old installation",
              detail:
                "Deletes the earlier installation's clinic data, images, and settings from this computer. Backups are kept.",
              run: () => bridge.PurgeResidue(),
            },
          };
    } catch (e) {
      result = { state: "bad", how: String(e) };
    }
    setResidue(result);
    return result;
  }, []);

  const checkDocker = useCallback(async (): Promise<Result> => {
    setDocker(WAITING);
    let result: Result;
    try {
      const status = await bridge.DockerStatus();
      if (status.ok) {
        result = { state: "ok", how: "" };
      } else {
        const plan = await bridge.DockerPlan();
        result = {
          state: "bad",
          how: status.message,
          action: actionFor(plan, () =>
            plan.action === "open" ? bridge.OpenDocker() : bridge.InstallDocker(),
          ),
        };
      }
    } catch (e) {
      result = { state: "bad", how: String(e) };
    }
    setDocker(result);
    return result;
  }, []);

  const checkGit = useCallback(async (): Promise<Result> => {
    setGit(WAITING);
    let result: Result;
    try {
      const status = await bridge.GitStatus();
      if (status.ok) {
        result = { state: "ok", how: "" };
      } else {
        const plan = await bridge.GitPlan();
        result = {
          state: "bad",
          how: status.message,
          action: actionFor(plan, () => bridge.InstallGit()),
        };
      }
    } catch (e) {
      result = { state: "bad", how: String(e) };
    }
    setGit(result);
    return result;
  }, []);

  const checkMDNS = useCallback(async (): Promise<Result> => {
    setMdns(WAITING);
    let result: Result;
    try {
      const status = await bridge.MDNSStatus();
      result = {
        state: status.ok ? "ok" : "bad",
        how: status.ok ? "" : status.message,
      };
    } catch (e) {
      result = { state: "bad", how: String(e) };
    }
    setMdns(result);
    return result;
  }, []);

  // Windows-only gate; elsewhere the host reports it as not applicable and the
  // row is hidden rather than shown as passing.
  const checkNetwork = useCallback(async (): Promise<Result | null> => {
    let result: Result | null;
    try {
      const status = await bridge.NetworkStatus();
      result = status.applicable
        ? {
            state: status.ok ? "ok" : "bad",
            how: status.ok ? "" : status.how || status.message,
            action:
              status.ok || !status.fixable
                ? undefined
                : {
                    label: "Fix automatically",
                    detail: "Sets this WiFi network to Private and opens the clinic's ports.",
                    run: () => bridge.FixNetwork(),
                  },
          }
        : null;
    } catch {
      result = null;
    }
    setNetwork(result);
    return result;
  }, []);

  const recheckAll = useCallback(async (): Promise<CheckTone> => {
    const [r, d, g, m, n] = await Promise.all([
      checkResidue(),
      checkDocker(),
      checkGit(),
      checkMDNS(),
      checkNetwork(),
    ]);
    return summarise(n ? [r, d, g, m, n] : [r, d, g, m]);
  }, [checkResidue, checkDocker, checkGit, checkMDNS, checkNetwork]);

  useEffect(() => {
    void recheckAll();
  }, [recheckAll]);

  const checks = useMemo<Check[]>(() => {
    const list: Check[] = [
      {
        id: "residue",
        title: "A clean computer",
        detail: "Nothing left from an earlier CARE Desktop",
        ...residue,
      },
      {
        id: "docker",
        title: "Docker",
        detail: "Runs the clinic software on this computer",
        ...docker,
      },
      {
        id: "git",
        title: "Git",
        detail: "Downloads the clinic software",
        ...git,
      },
      {
        id: "mdns",
        title: "Network name",
        detail: `${host} on the clinic WiFi`,
        ...mdns,
      },
    ];
    if (network) {
      list.push({
        id: "network",
        title: "Network profile",
        detail: "WiFi set to Private",
        ...network,
      });
    }
    return list;
  }, [docker, git, host, mdns, network, residue]);

  const overall = useMemo(
    () => summarise(network ? [residue, docker, git, mdns, network] : [residue, docker, git, mdns]),
    [docker, git, mdns, network, residue],
  );

  return { checks, overall, recheckAll, checkMDNS };
}
