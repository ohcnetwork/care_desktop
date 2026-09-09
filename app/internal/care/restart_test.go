package care

import (
	"runtime"
	"strings"
	"testing"
)

func TestRestartPlanIsCoherent(t *testing.T) {
	p := (&Engine{}).RestartPlan()
	if !p.Needed {
		// Nothing outstanding: the plan must be empty rather than half-filled,
		// since the UI keys the dialog off Needed alone.
		if p.Title != "" || p.Detail != "" || p.Label != "" {
			t.Fatalf("plan not needed but carries text: %+v", p)
		}
		return
	}
	for name, got := range map[string]string{"title": p.Title, "detail": p.Detail, "label": p.Label} {
		if strings.TrimSpace(got) == "" {
			t.Fatalf("plan is needed but has no %s: %+v", name, p)
		}
	}
	// The operator is being asked to restart; the reason has to be in the text.
	if !strings.Contains(strings.ToLower(p.Detail), "restart") {
		t.Fatalf("detail should explain the restart: %q", p.Detail)
	}
}

// macOS never needs a restart for Docker Desktop, so the wizard must never ask.
func TestRestartNeverNeededOnMacOS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin only")
	}
	if (&Engine{}).RestartPlan().Needed {
		t.Fatal("macOS should never be told to restart")
	}
}

func TestWindowsOnlyChecksHappenOnWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("these guards are for the other platforms")
	}
	if windowsRebootPending() {
		t.Fatal("windowsRebootPending must be false off Windows")
	}
	if dockerGroupPending() && runtime.GOOS != "linux" {
		t.Fatal("dockerGroupPending must be false off Linux")
	}
}

func TestContainsWord(t *testing.T) {
	// Substring matching would make "dockerroot" look like "docker".
	if !containsWord("wheel staff docker admin", "docker") {
		t.Fatal("should match a whole word")
	}
	if containsWord("wheel dockerroot admin", "docker") {
		t.Fatal("must not match inside another group name")
	}
	if containsWord("", "docker") {
		t.Fatal("empty group list matches nothing")
	}
}
