package prereq

import (
	"runtime"
	"testing"
)

func TestWSLCheckAlwaysOffersAWayForward(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("WSL only applies to Windows")
	}
	status := WSLCheck()
	if !status.Applicable {
		t.Fatal("WSL must be reported as applicable on Windows")
	}
	if status.OK {
		return
	}
	if !status.Fixable {
		t.Fatal("a failing WSL row with no fix leaves the operator stuck with no button to press")
	}
	if status.How == "" {
		t.Fatal("a failing WSL row must say what to do next")
	}
}

func TestWSLIsHiddenAwayFromWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("covered by the Windows case")
	}
	status := WSLCheck()
	if status.Applicable {
		t.Fatal("WSL must not be shown as a requirement away from Windows")
	}
	if !status.OK {
		t.Fatal("a hidden row must not drag the overall check to failing")
	}
}

func TestWSLNeedsBothTheAppAndThePlatform(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("WSL only applies to Windows")
	}
	vmPlatformOn.Store(false)
	defer vmPlatformOn.Store(false)

	if vmPlatformEnabled() != wslReady() && wslAnswers() {
		t.Fatal("readiness must follow the platform feature, not wsl --status alone")
	}
	if wslReady() && !vmPlatformEnabled() {
		t.Fatal("WSL reported ready while the Virtual Machine Platform is off")
	}
}

func TestVMPlatformProbeCachesOnlySuccess(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the probe is Windows-only")
	}
	vmPlatformOn.Store(false)
	defer vmPlatformOn.Store(false)

	if !vmPlatformEnabled() {
		t.Skip("platform is off on this machine; nothing to cache")
	}
	if !vmPlatformOn.Load() {
		t.Fatal("a positive probe must be cached so every check does not pay for it")
	}
}
