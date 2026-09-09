package care

import (
	"runtime"
	"strings"
	"testing"
)

// A plan is what the operator sees on a failing row, so the invariant that
// matters is: whenever there is an action, there is a button label and a
// sentence saying what pressing it does.
func assertPlanCoherent(t *testing.T, p ToolPlan) {
	t.Helper()
	if p.Action == ActionNone {
		return
	}
	if strings.TrimSpace(p.Label) == "" {
		t.Fatalf("%s plan has action %q but no button label", p.Tool, p.Action)
	}
	if strings.TrimSpace(p.Detail) == "" {
		t.Fatalf("%s plan has action %q but no explanation", p.Tool, p.Action)
	}
	if p.Action == ActionManual && p.URL == "" {
		t.Fatalf("%s plan sends the operator to a page but names no URL", p.Tool)
	}
}

func TestDockerPlanIsActionable(t *testing.T) {
	p := (&Engine{}).DockerPlan()
	if p.Tool != "docker" {
		t.Fatalf("wrong tool: %q", p.Tool)
	}
	assertPlanCoherent(t, p)
}

func TestGitPlanIsActionable(t *testing.T) {
	p := (&Engine{}).GitPlan()
	if p.Tool != "git" {
		t.Fatalf("wrong tool: %q", p.Tool)
	}
	assertPlanCoherent(t, p)
}

// The install plan must always offer something, on every OS: a machine we
// cannot install on still gets pointed at the vendor's page.
func TestDockerInstallPlanAlwaysOffersARoute(t *testing.T) {
	p := (&Engine{}).dockerInstallPlan()
	if p.Action != ActionInstall && p.Action != ActionManual {
		t.Fatalf("install plan should install or send to a page, got %q", p.Action)
	}
	assertPlanCoherent(t, p)
	if !strings.HasPrefix(p.URL, "https://") {
		t.Fatalf("fallback URL should be https, got %q", p.URL)
	}
}

func TestLinuxPackageManagerOnlyOnLinux(t *testing.T) {
	got := linuxPackageManager()
	if runtime.GOOS != "linux" && got != "" {
		t.Fatalf("no package manager should be reported on %s, got %q", runtime.GOOS, got)
	}
}

func TestHostOf(t *testing.T) {
	for url, want := range map[string]string{
		dockerDMGArm64: "desktop.docker.com",
		dockerEXEWin:   "desktop.docker.com",
		gitPageURL:     "git-scm.com",
	} {
		if got := hostOf(url); got != want {
			t.Fatalf("hostOf(%q) = %q, want %q", url, got, want)
		}
	}
}

func TestCurrentUsernameIsNotEmpty(t *testing.T) {
	// The macOS installer is handed this as --user=; an empty value would make
	// the elevated command silently wrong.
	if strings.TrimSpace(currentUsername()) == "" {
		t.Fatal("currentUsername() must resolve to something")
	}
}
