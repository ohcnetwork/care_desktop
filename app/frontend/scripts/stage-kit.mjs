// Stage the deployment kit into ../kit so Go can embed it (//go:embed all:kit).
// Runs at frontend build time, before the Go compile, so the embed is fresh.
// Keeps the repo's root files as the single source of truth — no duplication in git.
import { cpSync, mkdirSync, rmSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url)); // app/frontend/scripts
const root = join(here, "..", "..", ".."); // repo root
const kit = join(here, "..", "..", "kit"); // app/kit

const items = [
  "docker-compose.yml",
  "backup.Dockerfile",
  "caddy.Dockerfile",
  "backend.env",
  "frontend.env",
  "clinic_settings.py",
  "Caddyfile",
  "versions.env",
  "minio",
  "scripts",
];

// Re-stage each item (preserve kit/.gitkeep, which is the only tracked file here).
mkdirSync(kit, { recursive: true });
for (const item of items) {
  rmSync(join(kit, item), { recursive: true, force: true });
  cpSync(join(root, item), join(kit, item), { recursive: true });
}
console.log(`staged ${items.length} kit entries → app/kit`);
