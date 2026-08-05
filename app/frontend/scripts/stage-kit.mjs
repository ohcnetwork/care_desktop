// Stage the deployment kit into ../kit so Go can embed it (//go:embed all:kit).
// Runs at frontend build time, before the Go compile, so the embed is fresh.
// Keeps the repo's root files as the single source of truth — no duplication in git.
import { cpSync, existsSync, mkdirSync, readdirSync, rmSync } from "node:fs";
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
  "tls.env",
  "versions.env",
  "apps.json",
  "minio",
  "scripts",
];

// tls.env is staged from the blank template, ALWAYS — never from the developer's own
// tls.env, which holds a live Cloudflare API token and would otherwise be embedded in
// every shipped binary. The real file is a runtime artifact of an install, written by
// the wizard and preserved across kit refreshes; it is never a build input.
const sources = { "tls.env": "tls.env.example" };

// Re-stage each item (preserve kit/.gitkeep, which is the only tracked file here).
mkdirSync(kit, { recursive: true });
for (const item of items) {
  const src = join(root, sources[item] ?? item);
  if (!existsSync(src)) throw new Error(`stage-kit: missing ${src}`);
  rmSync(join(kit, item), { recursive: true, force: true });
  cpSync(src, join(kit, item), { recursive: true });
}

// Drop anything left over from an earlier layout. Without this a file that used to be
// staged (a removed Caddyfile variant, say) lingers in the working tree and gets
// embedded forever, since go:embed takes whatever is in the directory.
const keep = new Set([...items, ".gitkeep"]);
for (const entry of readdirSync(kit)) {
  if (keep.has(entry)) continue;
  rmSync(join(kit, entry), { recursive: true, force: true });
  console.log(`pruned stale kit entry: ${entry}`);
}
console.log(`staged ${items.length} kit entries → app/kit`);
