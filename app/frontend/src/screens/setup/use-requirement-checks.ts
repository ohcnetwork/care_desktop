import { useCallback, useEffect, useMemo, useState } from "react";

import { bridge } from "@/lib/bridge";

export type CheckTone = "wait" | "ok" | "bad";
export type Check = {
  id: "runtime" | "mdns" | "network";
  title: string;
  detail: string;
  state: CheckTone;
  how: string;
  fixable: boolean;
};

type Result = { state: CheckTone; how: string; fixable: boolean };

const WAITING: Result = { state: "wait", how: "", fixable: false };

function summarise(results: Result[]): CheckTone {
  if (results.some((r) => r.state === "bad")) return "bad";
  return results.some((r) => r.state === "wait") ? "wait" : "ok";
}

/**
 * The three gates on "can this computer run a clinic". The network profile one
 * is Windows-only: elsewhere the host reports it as not applicable and it is
 * hidden rather than shown as passing.
 */
export function useRequirementChecks(host: string) {
  const [runtime, setRuntime] = useState<Result>(WAITING);
  const [mdns, setMdns] = useState<Result>(WAITING);
  const [network, setNetwork] = useState<Result | null>(null);

  const checkRuntime = useCallback(async (): Promise<Result> => {
    setRuntime(WAITING);
    let result: Result;
    try {
      const [docker, git] = await Promise.all([bridge.DockerStatus(), bridge.GitStatus()]);
      const ok = docker.ok && git.ok;
      result = {
        state: ok ? "ok" : "bad",
        how: ok ? "" : docker.ok ? git.message : docker.message,
        fixable: false,
      };
    } catch (e) {
      result = { state: "bad", how: String(e), fixable: false };
    }
    setRuntime(result);
    return result;
  }, []);

  const checkMDNS = useCallback(async (): Promise<Result> => {
    setMdns(WAITING);
    let result: Result;
    try {
      const status = await bridge.MDNSStatus();
      result = {
        state: status.ok ? "ok" : "bad",
        how: status.ok ? "" : status.how || status.message,
        fixable: false,
      };
    } catch (e) {
      result = { state: "bad", how: String(e), fixable: false };
    }
    setMdns(result);
    return result;
  }, []);

  const checkNetwork = useCallback(async (): Promise<Result | null> => {
    let result: Result | null;
    try {
      const status = await bridge.NetworkStatus();
      result = status.applicable
        ? {
            state: status.ok ? "ok" : "bad",
            how: status.ok ? "" : status.how || status.message,
            fixable: status.fixable,
          }
        : null;
    } catch {
      result = null;
    }
    setNetwork(result);
    return result;
  }, []);

  const recheckAll = useCallback(async (): Promise<CheckTone> => {
    const [r, m, n] = await Promise.all([checkRuntime(), checkMDNS(), checkNetwork()]);
    return summarise(n ? [r, m, n] : [r, m]);
  }, [checkMDNS, checkNetwork, checkRuntime]);

  const fixNetwork = useCallback(async (): Promise<string> => {
    let message = "Network set to Private.";
    try {
      await bridge.FixNetwork();
    } catch (e) {
      message = `Couldn't change the network: ${String(e)}`;
    }
    await checkNetwork();
    return message;
  }, [checkNetwork]);

  useEffect(() => {
    void recheckAll();
  }, [recheckAll]);

  const checks = useMemo<Check[]>(() => {
    const list: Check[] = [
      {
        id: "runtime",
        title: "Runtime engine",
        detail: "Docker, Compose and Git",
        ...runtime,
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
  }, [host, mdns, network, runtime]);

  const overall = useMemo(() => summarise(network ? [runtime, mdns, network] : [runtime, mdns]), [mdns, network, runtime]);

  return { checks, overall, recheckAll, checkMDNS, fixNetwork };
}
