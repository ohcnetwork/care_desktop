// Stage deployments/ into ../install so Go can embed it (//go:embed all:install).
// Runs at frontend build time, before the Go compile, so the embed is fresh.
// deployments/ is the single source of truth; nothing is duplicated in git.
//
// The list is read from the directory rather than hardcoded: a hand-maintained
// array silently drifts, and a file added to the stack but missed here goes
// missing at runtime on the clinic's machine, not at build time here.
import { cpSync, mkdirSync, readdirSync, rmSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url)); // app/frontend/scripts
const source = join(here, "..", "..", "..", "deployments");
const install = join(here, "..", "..", "install"); // app/install

const items = readdirSync(source).filter((name) => !name.startsWith("."));
if (items.length === 0) throw new Error(`nothing to stage: ${source} is empty`);

// Clear first, so a file deleted from deployments/ cannot linger in an install
// dir staged by an earlier build. .gitkeep is tracked and must survive.
mkdirSync(install, { recursive: true });
for (const stale of readdirSync(install)) {
  if (stale !== ".gitkeep") rmSync(join(install, stale), { recursive: true, force: true });
}
for (const item of items) {
  cpSync(join(source, item), join(install, item), { recursive: true });
}
console.log(`staged ${items.length} entries → app/install`);
