// Keep only woff2 fonts in dist. The bundled icon/font CSS declares woff2 + woff +
// ttf + svg @font-face sources; every Wails webview (WKWebView, WebView2,
// WebKitGTK) supports woff2 and picks it first, so the legacy formats are never
// fetched at runtime — they'd only bloat the embedded binary (~15 MB → ~1 MB).
import { readdirSync, rmSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const assets = join(dirname(fileURLToPath(import.meta.url)), "..", "dist", "assets");
let removed = 0;
let freed = 0;
for (const f of readdirSync(assets)) {
  if (/\.(woff|ttf|svg)$/i.test(f)) {
    // note: `.woff2` does NOT match `\.woff$` (trailing "2"), so it's kept.
    const p = join(assets, f);
    freed += statSync(p).size;
    rmSync(p);
    removed++;
  }
}
console.log(`pruned ${removed} legacy font files (${(freed / 1e6).toFixed(1)} MB) — kept woff2`);
