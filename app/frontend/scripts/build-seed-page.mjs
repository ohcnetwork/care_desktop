import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const page = join(here, "..", "..", "..", "data-loading", "frontend");
if (!existsSync(join(page, "package.json"))) {
  throw new Error(`seed page source not found at ${page}`);
}

const run = (args) => {
  const result = spawnSync(process.platform === "win32" ? "npm.cmd" : "npm", args, {
    cwd: page,
    stdio: "inherit",
    shell: process.platform === "win32",
  });
  if (result.status !== 0) process.exit(result.status ?? 1);
};

if (!existsSync(join(page, "node_modules"))) run(["ci", "--no-audit", "--no-fund"]);
run(["run", "build"]);
