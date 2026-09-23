package prereq

import (
	"runtime"
	"strings"
	"testing"
)

// Windows only lets administrators write under HKCU\Software\Policies, so a
// profile put there never lands and Rancher Desktop falls back to its welcome
// dialog and a Kubernetes download.
func TestRancherProfileKeyAvoidsThePoliciesTree(t *testing.T) {
	if strings.Contains(strings.ToLower(rancherProfileKey), `\policies\`) {
		t.Fatalf("profile key needs elevation to write: %s", rancherProfileKey)
	}
	if !strings.HasSuffix(rancherProfileKey, `\Defaults`) {
		t.Fatalf("Rancher Desktop reads defaults from a Defaults subkey, got %s", rancherProfileKey)
	}
}

func TestRancherLaunchArgsSuppressDialogsAndKubernetes(t *testing.T) {
	args := rancherLaunchArgs()
	if args[0] != "--no-modal-dialogs" {
		t.Fatalf("first argument = %q, want --no-modal-dialogs", args[0])
	}
	if !strings.Contains(strings.Join(args, " "), "--kubernetes.enabled=false") {
		t.Fatalf("launch arguments must keep Kubernetes off: %v", args)
	}
	if len(rancherLaunchArgs()) != len(rancherSettings())+1 {
		t.Fatal("rancherLaunchArgs must pass every setting plus --no-modal-dialogs")
	}
}

// rdctl rejects --application.admin-access on Windows, and Rancher Desktop
// refuses to start when it is handed one. It is a Unix privileged-helper
// setting, so it must never reach the Windows command line or profile.
func TestAdminAccessIsUnixOnly(t *testing.T) {
	joined := strings.Join(rancherLaunchArgs(), " ")
	if runtime.GOOS == "windows" {
		if strings.Contains(joined, "admin-access") {
			t.Fatalf("Windows cannot accept admin-access: %s", joined)
		}
		return
	}
	if !strings.Contains(joined, "--application.admin-access=true") {
		t.Fatalf("admin access is still needed away from Windows: %s", joined)
	}
}

func TestRancherSettingsAreNotSharedBetweenCalls(t *testing.T) {
	first := rancherSettings()
	first[0] = "--mutated"
	if rancherSettings()[0] == "--mutated" {
		t.Fatal("callers can corrupt the settings used by every later launch")
	}
}
