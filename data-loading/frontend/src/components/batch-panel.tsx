import { Progress } from "@/components/ui/progress";
import type { BatchProgress } from "@/lib/batch";

export function BatchPanel({ title, progress, running }: { title: string; progress: BatchProgress; running: boolean }) {
  const pct = progress.total ? Math.round((progress.done / progress.total) * 100) : 100;
  return (
    <div className="rounded-xl border border-line bg-white px-4 py-3.5">
      <div className="mb-2 flex items-baseline justify-between gap-3">
        <span className="text-[13.5px] font-semibold text-ink">{title}</span>
        <span className="font-mono text-[12.5px] text-muted-foreground">
          {progress.done}/{progress.total}
        </span>
      </div>
      <Progress value={pct} />
      <div className="mt-2.5 flex flex-wrap gap-x-4 gap-y-1 text-[12.5px] text-muted-foreground">
        <span>{progress.created} created</span>
        <span>{progress.skipped} already present</span>
        <span className={progress.failed ? "text-danger-ink" : undefined}>{progress.failed} failed</span>
        {running ? <span>working…</span> : null}
      </div>
      {progress.failures.length ? (
        <ul className="mt-2.5 max-h-40 space-y-1 overflow-auto rounded-md bg-danger-tint px-3 py-2 text-[12.5px] text-danger-ink">
          {progress.failures.map((f, i) => (
            <li key={i}>
              <span className="font-semibold">{f.label}</span>: {f.reason}
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
