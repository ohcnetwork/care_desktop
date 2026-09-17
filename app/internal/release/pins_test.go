package release

import (
	"os"
	"strings"
	"testing"
)

func deploymentPins(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../../deployments/.env")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDeploymentSourcesAreImmutable(t *testing.T) {
	pins, err := Load(deploymentPins(t))
	if err != nil {
		t.Fatal(err)
	}
	if !IsCommitRef(pins.BeRef) || !IsCommitRef(pins.FeRef) {
		t.Fatal("deployment source refs are not immutable commits")
	}
}

func TestLoadRequiresImmutableReleaseSources(t *testing.T) {
	data := string(deploymentPins(t))
	pins, err := Load([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"CARE_BE_REF", "CARE_FE_REF"} {
		t.Run(key, func(t *testing.T) {
			ref := pins.BeRef
			if key == "CARE_FE_REF" {
				ref = pins.FeRef
			}
			moving := strings.Replace(data, key+"="+ref, key+"=develop", 1)
			if _, err := Load([]byte(moving)); err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("moving release source was accepted: %v", err)
			}
			development := strings.Replace(moving, "CARE_DESKTOP_VERSION="+pins.AppVersion, "CARE_DESKTOP_VERSION="+pins.AppVersion+"-dev", 1)
			if _, err := Load([]byte(development)); err != nil {
				t.Fatalf("explicit development source was rejected: %v", err)
			}
		})
	}
	for _, version := range []string{"0.0", "v0.1.0", "0.1.0-release", "01.1.0"} {
		t.Run(version, func(t *testing.T) {
			invalid := strings.Replace(data, "CARE_DESKTOP_VERSION="+pins.AppVersion, "CARE_DESKTOP_VERSION="+version, 1)
			if _, err := Load([]byte(invalid)); err == nil {
				t.Fatal("invalid version was accepted")
			}
		})
	}
}

func TestIsCommitRef(t *testing.T) {
	for ref, want := range map[string]bool{
		"a749b92794ac175db8839d3d75ca36402a196282": true,
		"A749B92794AC175DB8839D3D75CA36402A196282": true,
		"a749b927": false,
		"develop":  false,
		"v0.1.0":   false,
		"g749b92794ac175db8839d3d75ca36402a196282": false,
	} {
		if got := IsCommitRef(ref); got != want {
			t.Errorf("IsCommitRef(%q) = %v, want %v", ref, got, want)
		}
	}
}
