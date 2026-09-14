package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const autostartLabel = "ohc.care-desktop"
const autostartName = "CARE Desktop"

func (a *App) macPlistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", autostartLabel+".plist")
}

func (a *App) linuxDesktopPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "autostart", "care-desktop.desktop")
}

// AutostartEnabled reports whether the app is set to launch at login.
func (a *App) AutostartEnabled() bool {
	switch runtime.GOOS {
	case "darwin":
		_, err := os.Stat(a.macPlistPath())
		return err == nil
	case "linux":
		_, err := os.Stat(a.linuxDesktopPath())
		return err == nil
	case "windows":
		// reg query exits 0 only if the value exists.
		return exec.Command("reg", "query",
			`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", autostartName).Run() == nil
	}
	return false
}

// SetAutostart turns launch-at-login on or off, passing --autostart so the app
// can bring CARE up by itself.
func (a *App) SetAutostart(on bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	switch runtime.GOOS {
	case "darwin":
		if !on {
			return removeIfExists(a.macPlistPath())
		}
		plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>` + autostartLabel + `</string>
  <key>ProgramArguments</key><array><string>` + exe + `</string><string>--autostart</string></array>
  <key>RunAtLoad</key><true/>
</dict></plist>
`
		if err := os.MkdirAll(filepath.Dir(a.macPlistPath()), 0o755); err != nil {
			return err
		}
		return os.WriteFile(a.macPlistPath(), []byte(plist), 0o644)
	case "linux":
		if !on {
			return removeIfExists(a.linuxDesktopPath())
		}
		desktop := "[Desktop Entry]\nType=Application\nName=" + autostartName +
			"\nExec=\"" + exe + "\" --autostart\nX-GNOME-Autostart-enabled=true\n"
		if err := os.MkdirAll(filepath.Dir(a.linuxDesktopPath()), 0o755); err != nil {
			return err
		}
		return os.WriteFile(a.linuxDesktopPath(), []byte(desktop), 0o644)
	case "windows":
		key := `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
		if !on {
			return exec.Command("reg", "delete", key, "/v", autostartName, "/f").Run()
		}
		return exec.Command("reg", "add", key, "/v", autostartName, "/t", "REG_SZ",
			"/d", `"`+exe+`" --autostart`, "/f").Run()
	}
	return nil
}

func removeIfExists(p string) error {
	if _, err := os.Stat(p); err != nil {
		return nil
	}
	return os.Remove(p)
}
