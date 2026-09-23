package prereq

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

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
		const key = `HKCU\Software\Policies\Rancher Desktop\Defaults`
		args := [][]string{
			{key, "/v", "version", "/t", "REG_DWORD", "/d", fmt.Sprint(rancherProfileVersion)},
			{key + `\application`, "/v", "adminAccess", "/t", "REG_DWORD", "/d", "1"},
			{key + `\application`, "/v", "autoStart", "/t", "REG_DWORD", "/d", "1"},
			{key + `\application`, "/v", "startInBackground", "/t", "REG_DWORD", "/d", "1"},
			{key + `\containerEngine`, "/v", "name", "/t", "REG_SZ", "/d", "moby"},
			{key + `\kubernetes`, "/v", "enabled", "/t", "REG_DWORD", "/d", "0"},
		}
		for _, a := range args {
			if err := proc.Command("reg", append([]string{"add"}, append(a, "/f")...)...).Run(); err != nil {
				return fmt.Errorf("could not write the Rancher Desktop profile: %w", err)
			}
		}
	default:
		return nil
	}
	applyRancherProfileNow()
	return nil
}

var rancherSettings = []string{
	"--application.admin-access=true",
	"--application.auto-start=true",
	"--application.start-in-background=true",
	"--container-engine.name=moby",
	"--kubernetes.enabled=false",
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
