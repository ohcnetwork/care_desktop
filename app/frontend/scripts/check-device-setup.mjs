import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";

const html = readFileSync(new URL("../../../deployments/setup/index.html", import.meta.url), "utf8");
const script = html.match(/<script>([\s\S]*?)<\/script>/)[1];
const ids = [...html.matchAll(/\bid="([^"]+)"/g)].map((match) => match[1]);
assert.equal(new Set(ids).size, ids.length, "IDs must be unique");

function load(userAgent, hash = "", maxTouchPoints = 0) {
  const elements = Object.fromEntries(ids.map((id) => [id, {}]));
  const events = {};
  const context = {
    navigator: { userAgent, maxTouchPoints },
    location: { hash },
    document: {
      getElementById(id) {
        assert.ok(elements[id], `Missing element: ${id}`);
        return elements[id];
      },
    },
    window: { addEventListener: (name, handler) => { events[name] = handler; } },
  };
  runInNewContext(script, context);
  return { elements, context, events };
}

for (const [ua, touch, expected] of [
  ["Mozilla/5.0 (Windows NT 10.0; Win64; x64)", 0, "windows"],
  ["Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)", 0, "macos"],
  ["Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X)", 5, "ios"],
  ["Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15)", 5, "ios"],
  ["Mozilla/5.0 (Linux; Android 14)", 5, "android"],
  ["Mozilla/5.0 (X11; Linux x86_64)", 0, "linux"],
  ["Mozilla/5.0 (X11; CrOS x86_64)", 0, ""],
  ["unknown", 0, ""],
]) {
  const { elements } = load(ua, "", touch);
  assert.equal(elements.device.value, expected, ua);
  assert.equal(elements.guide.hidden, !expected);
  assert.equal(elements["device-picker"].hidden, false);
  if (!expected) continue;
  assert.equal(elements.download.href, expected === "linux" ? "/setup/install-cert.sh" : "/root.crt?ok=1");
  assert.equal(elements.download.download, expected === "linux" ? "install-cert.sh" : "root.crt");
  assert.equal(elements["linux-command"].hidden, expected !== "linux");
  assert.equal(elements.advanced.hidden, ["ios", "android"].includes(expected));
  assert.equal(elements.advanced.open, false);
  assert.match(elements.instructions.innerHTML, /<li>.+<\/li>/);
}

const { elements, context, events } = load("Windows", "#ios");
assert.equal(elements.device.value, "ios", "Explicit choice overrides detection");
assert.match(elements.instructions.innerHTML, /Certificate Trust Settings/);
elements.device.value = "linux";
elements.device.onchange();
assert.equal(context.location.hash, "linux");
assert.equal(elements["linux-manual"].hidden, false);
assert.equal(elements.installer.hidden, true);
elements.advanced.open = true;
context.location.hash = "#windows";
events.hashchange();
assert.equal(elements.device.value, "windows");
assert.equal(elements.advanced.open, false);
assert.equal(elements["linux-manual"].hidden, true);
assert.equal(elements.installer.hidden, false);
assert.equal(elements["installer-download"].href, "/setup/install-cert.ps1");
assert.equal(elements["installer-command"].hidden, true);
for (const hash of ["#unknown", "#__proto__", "#constructor", "#<script>"]) {
  assert.equal(load("Windows", hash).elements.guide.hidden, true);
}
assert.match(html, /href="https:\/\/example.local" target="_blank" rel="noopener"/);
assert.match(html, /Do not bypass the warning/);
assert.match(html, /<noscript>/);
console.log("Device setup checks passed.");
