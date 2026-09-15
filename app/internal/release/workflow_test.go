package release

import (
	"testing"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

func TestReleaseWorkflowValidatesIdentity(t *testing.T) {
	if !proc.Exists("node") {
		t.Skip("Node.js is not installed")
	}
	script := `
const fs = require("fs");
const vm = require("vm");
const workflow = fs.readFileSync(".github/workflows/release.yml", "utf8");
const match = workflow.match(/          node <<'NODE'\n([\s\S]*?)\n          NODE/);
if (!match) throw new Error("Release identity validator is missing");
const validator = match[1].replace(/^          /gm, "");
const pins = fs.readFileSync("deployments/.env", "utf8");
const metadata = JSON.parse(fs.readFileSync("app/wails.json", "utf8"));
const version = metadata.info.productVersion;
const change = (key, value) => pins.replace(new RegExp("^" + key + "=.*$", "m"), key + "=" + value);
const cases = [
  {name: "release", ref: "refs/tags/v" + version, tag: "v" + version},
  {name: "development packaging", ref: "refs/heads/main", tag: "main"},
  {name: "tag mismatch", tag: "v9.9.9", fails: true},
  {name: "metadata mismatch", version: "9.9.9", fails: true},
  {name: "moving backend", pins: change("CARE_BE_REF", "develop"), fails: true},
  {name: "moving frontend", pins: change("CARE_FE_REF", "develop"), fails: true},
  {name: "development version", pins: change("CARE_DESKTOP_VERSION", version + "-dev"), fails: true},
  {name: "missing version", pins: pins.replace(/^CARE_DESKTOP_VERSION=.*\n/m, ""), fails: true},
  {name: "duplicate version", pins: pins + "\nCARE_DESKTOP_VERSION=" + version + "\n", fails: true},
];
for (const test of cases) {
  let output = "";
  let failed = false;
  const input = JSON.stringify({info: {...metadata.info, productVersion: test.version || version}});
  try {
    vm.runInNewContext(validator, {
      require: name => {
        if (name !== "fs") throw new Error("Unexpected module");
        return {
          readFileSync: path => {
            if (path === "deployments/.env") return test.pins === undefined ? pins : test.pins;
            if (path === "app/wails.json") return input;
            throw new Error("Unexpected input path");
          },
          appendFileSync: (path, value) => {
            if (path !== "output") throw new Error("Unexpected output path");
            output += value;
          },
        };
      },
      process: {env: {
        GITHUB_REF: test.ref || "refs/tags/v" + version,
        GITHUB_REF_NAME: test.tag || "v" + version,
        GITHUB_OUTPUT: "output",
      }},
    });
  } catch (error) {
    failed = true;
  }
  if (failed !== Boolean(test.fails)) throw new Error(test.name + ": unexpected validation result");
  if (!failed && output !== "version=" + version + "\n") throw new Error(test.name + ": incorrect artifact version");
}
`
	cmd := proc.Command("node", "-e", script)
	cmd.Dir = "../../.."
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("release workflow validation failed: %v\n%s", err, output)
	}
}
