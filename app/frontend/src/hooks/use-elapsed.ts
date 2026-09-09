import { useEffect, useState } from "react";

/** Seconds since `startedAt`, ticking once a second while `running`. */
export function useElapsed(startedAt: number, running: boolean): number {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!running || !startedAt) return;
    setNow(Date.now());
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, [running, startedAt]);

  return startedAt ? Math.max(0, Math.floor((now - startedAt) / 1000)) : 0;
}
