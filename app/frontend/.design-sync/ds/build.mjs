import { execFileSync } from "node:child_process";
import { mkdirSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const frontend = join(here, "..", "..");
const pkg = join(here, "..", ".cache", "pkg");
const bin = (n) => join(frontend, "node_modules", ".bin", n);

execFileSync(bin("vite"), ["build", "--config", join(here, "vite.config.mjs"), "--logLevel", "warn"], { cwd: frontend, stdio: "inherit" });
execFileSync(bin("tsc"), ["-p", join(here, "tsconfig.json")], { cwd: frontend, stdio: "inherit" });

const types = join(pkg, "dist", "types");
const srcTypes = join(types, "src");
const walk = (d) => readdirSync(d).flatMap((f) => (statSync(join(d, f)).isDirectory() ? walk(join(d, f)) : [join(d, f)]));
for (const file of walk(types).filter((f) => f.endsWith(".d.ts"))) {
  const text = readFileSync(file, "utf8").replace(/(["'])@\/([^"']+)\1/g, (_, q, p) => {
    let rel = relative(dirname(file), join(srcTypes, p));
    if (!rel.startsWith(".")) rel = `./${rel}`;
    return `${q}${rel}${q}`;
  });
  writeFileSync(file, text);
}

const entryDts = join(types, ".design-sync", "ds", "index.d.ts");
writeFileSync(join(types, "index.d.ts"), readFileSync(entryDts, "utf8").replaceAll('"../../src/', '"./src/'));
rmSync(join(types, ".design-sync"), { recursive: true, force: true });

mkdirSync(pkg, { recursive: true });
writeFileSync(
  join(pkg, "package.json"),
  JSON.stringify(
    {
      name: "care-desktop-ui",
      version: JSON.parse(readFileSync(join(frontend, "..", "wails.json"), "utf8")).info?.productVersion ?? "0.0.0",
      type: "module",
      module: "dist/index.js",
      types: "dist/types/index.d.ts",
    },
    null,
    2,
  ),
);
console.log(`built ${relative(frontend, pkg)}`);
