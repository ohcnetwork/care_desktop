export type Outcome = "created" | "skipped" | "failed";

export type BatchProgress = {
  total: number;
  done: number;
  created: number;
  skipped: number;
  failed: number;
  failures: { label: string; reason: string }[];
};

export type BatchReport = BatchProgress;

export function emptyProgress(total: number): BatchProgress {
  return { total, done: 0, created: 0, skipped: 0, failed: 0, failures: [] };
}

export async function runBatch<T>(
  items: T[],
  label: (item: T) => string,
  work: (item: T) => Promise<Outcome>,
  onProgress: (p: BatchProgress) => void,
  concurrency = 4,
): Promise<BatchReport> {
  const progress = emptyProgress(items.length);
  let next = 0;
  const worker = async () => {
    while (next < items.length) {
      const item = items[next++];
      try {
        const outcome = await work(item);
        progress[outcome]++;
      } catch (e) {
        progress.failed++;
        progress.failures.push({ label: label(item), reason: e instanceof Error ? e.message : String(e) });
      }
      progress.done++;
      onProgress({ ...progress, failures: [...progress.failures] });
    }
  };
  await Promise.all(Array.from({ length: Math.min(concurrency, items.length) }, worker));
  return progress;
}
