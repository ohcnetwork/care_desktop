import { readdirSync, rmSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const assets = join(dirname(fileURLToPath(import.meta.url)), "..", "..", "..", "deployments", "seed-data", "assets");
let removed = 0;
let freed = 0;
for (const f of readdirSync(assets)) {
  if (/\.(woff|ttf|svg)$/i.test(f)) {
    const p = join(assets, f);
    freed += statSync(p).size;
    rmSync(p);
    removed++;
  }
}
console.log(`pruned ${removed} legacy font files (${(freed / 1e6).toFixed(1)} MB)`);
