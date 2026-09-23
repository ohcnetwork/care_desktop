package elevate

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

func TestUnixStepsPreserveEarlierFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell fixture")
	}
	cmd := proc.Command("sh", "-c", stepScript([]Step{
		{Sh: "exit 23"},
		{Sh: "printf 'must not run'"},
	}, false))
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 23 || len(out) != 0 {
		t.Fatalf("first failure was masked: %v, %q", err, out)
	}
}

func TestUnixStepsGroupCompoundCommands(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell fixture")
	}
	cmd := proc.Command("sh", "-c", stepScript([]Step{
		{Sh: "true || false"},
		{Sh: "printf 'second'"},
	}, false))
	out, err := cmd.CombinedOutput()
	if err != nil || string(out) != "second" {
		t.Fatalf("steps were not independently grouped: %v, %q", err, out)
	}
}

func TestWindowsStepsReportChildAndCommandFailures(t *testing.T) {
	inner := stepScript([]Step{{PS: "Add-Content -Path 'fixture' -Value 'value'"}, {PS: "certutil -addstore Root 'fixture'"}}, true)
	if !strings.Contains(inner, "$ErrorActionPreference = 'Stop'") ||
		strings.Count(inner, "if (-not $?) { exit 1 }") != 2 {
		t.Fatalf("step failures are not propagated: %s", inner)
	}
	outer := elevatedPS(inner)
	for _, required := range []string{"-Wait -PassThru", "-WindowStyle Hidden", "exit $p.ExitCode", PSQuote(inner)} {
		if !strings.Contains(outer, required) {
			t.Errorf("elevated script is missing %q: %s", required, outer)
		}
	}
}

func TestEmptyStepsDoNotElevate(t *testing.T) {
	if err := Steps(nil); err != nil {
		t.Fatal(err)
	}
}
