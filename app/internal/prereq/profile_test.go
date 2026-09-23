package prereq

import (
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
	if len(rancherLaunchArgs()) != len(rancherSettings)+1 {
		t.Fatal("rancherLaunchArgs grew the shared rancherSettings slice")
	}
}
