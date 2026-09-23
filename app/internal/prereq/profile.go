package prereq

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

// Rancher Desktop reads a deployment profile before its first run. A profile
// that sets anything at all also tells it to skip the welcome dialog, so CARE
// writes one before Rancher Desktop is ever started - not only when CARE is the
// one installing it.
const rancherProfileVersion = 18

const rancherProfilePlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>version</key><integer>%d</integer>
	<key>application</key>
	<dict>
		<key>adminAccess</key><true/>
		<key>autoStart</key><true/>
		<key>startInBackground</key><true/>
		<key>pathManagementStrategy</key><string>rcfiles</string>
	</dict>
	<key>containerEngine</key>
	<dict><key>name</key><string>moby</string></dict>
	<key>kubernetes</key>
	<dict><key>enabled</key><false/></dict>
</dict>
</plist>
`

// Rancher Desktop reads its registry profile from "Policies\Rancher Desktop"
// and then from "Rancher Desktop\Profile", and the later one wins. Windows
// keeps HKCU\Software\Policies read-only for anyone who isn't an administrator,
// so CARE writes the second, which needs no elevation.
const rancherProfileKey = `HKCU\Software\Rancher Desktop\Profile\Defaults`

func writeRancherProfile() error {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		dir := filepath.Join(home, "Library", "Preferences")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		body := fmt.Sprintf(rancherProfilePlist, rancherProfileVersion)
		path := filepath.Join(dir, "io.rancherdesktop.profile.defaults.plist")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
	case "windows":
		if err := writeRancherProfileWindows(); err != nil {
			return err
		}
	default:
		return nil
	}
	applyRancherProfileNow()
	return nil
}

func writeRancherProfileWindows() error {
	if rancherProfileWritten() {
		return nil
	}
	args := [][]string{
		{rancherProfileKey, "/v", "version", "/t", "REG_DWORD", "/d", fmt.Sprint(rancherProfileVersion)},
		{rancherProfileKey + `\application`, "/v", "adminAccess", "/t", "REG_DWORD", "/d", "1"},
		{rancherProfileKey + `\application`, "/v", "autoStart", "/t", "REG_DWORD", "/d", "1"},
		{rancherProfileKey + `\application`, "/v", "startInBackground", "/t", "REG_DWORD", "/d", "1"},
		{rancherProfileKey + `\containerEngine`, "/v", "name", "/t", "REG_SZ", "/d", "moby"},
		{rancherProfileKey + `\kubernetes`, "/v", "enabled", "/t", "REG_DWORD", "/d", "0"},
	}
	for _, a := range args {
		if err := proc.Command("reg", append([]string{"add"}, append(a, "/f")...)...).Run(); err != nil {
			return fmt.Errorf("could not write the Rancher Desktop profile: %w", err)
		}
	}
	return nil
}

// rancherProfileWritten reports whether this profile is already in the registry,
// so start-up doesn't shell out to reg six times on every launch.
func rancherProfileWritten() bool {
	out, err := proc.Command("reg", "query", rancherProfileKey, "/v", "version").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), fmt.Sprintf("0x%x", rancherProfileVersion))
}

var rancherSettings = []string{
	"--application.admin-access=true",
	"--application.auto-start=true",
	"--application.start-in-background=true",
	"--container-engine.name=moby",
	"--kubernetes.enabled=false",
}

// rancherLaunchArgs are what Rancher Desktop itself accepts on its command line.
// The profile only seeds a first run; these also settle an install that someone
// already answered the welcome dialog for.
func rancherLaunchArgs() []string {
	return append([]string{"--no-modal-dialogs"}, rancherSettings...)
}

func applyRancherProfileNow() {
	rdctl := rdctlPath()
	if rdctl == "" {
		return
	}
	_ = proc.Command(rdctl, append([]string{"set"}, rancherSettings...)...).Run()
}

func rdctlPath() string {
	var bundled string
	switch runtime.GOOS {
	case "darwin":
		bundled = filepath.Join(rancherAppMac, "Contents", "Resources", "resources", "darwin", "bin", "rdctl")
	case "windows":
		if exe := windowsRancherDesktopExe(); exe != "" {
			bundled = filepath.Join(filepath.Dir(exe), "resources", "resources", "win32", "bin", "rdctl.exe")
		}
	}
	if bundled != "" {
		if _, err := os.Stat(bundled); err == nil {
			return bundled
		}
	}
	if p, err := exec.LookPath("rdctl"); err == nil {
		return p
	}
	return ""
}
