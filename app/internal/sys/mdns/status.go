package mdns

import (
	"net"
	"runtime"
)

// NameStatus reports whether this machine is reachable as <name>.local, with a
// per-OS "how" the wizard shows when it isn't.
type NameStatus struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	How     string `json:"how"`
}

// Check verifies that <name>.local actually resolves right now - a real
// functional test (does the LAN answer?), uniform across OSes. It's gated in the
// installer because the frontend is baked to http://care.local. The "how" text when
// it fails depends on the mDNS mode. Note: in "advertise" mode the app answers this
// itself, so the app's status reports green as soon as its responder is up.
// name is the bare label (e.g. "care"); mode is one of advertise, rename, off.
func Check(name, mode string) NameStatus {
	full := name + ".local"
	if _, err := net.LookupHost(full); err == nil {
		return NameStatus{OK: true, Message: full + " resolves"}
	}
	switch mode {
	case "rename":
		return renameHow(name, full)
	case "off":
		return NameStatus{OK: false,
			Message: full + " not advertised (mDNS is off)",
			How:     "You're on static-IP mode. Open https://<server-ip>/ instead of " + full + ", or set CARE_MDNS_MODE=advertise."}
	default: // advertise
		how := "Open (and keep open) the CARE Desktop app - it advertises " + full +
			" on the LAN while running. Then re-check."
		if runtime.GOOS == "windows" {
			how += "\nOn Windows, also allow inbound UDP 5353 (PowerShell as Admin):\n" +
				"  Set-NetConnectionProfile -NetworkCategory Private\n" +
				"  New-NetFirewallRule -DisplayName \"mDNS\" -Direction Inbound -Protocol UDP -LocalPort 5353 -Action Allow -Profile Private"
		}
		how += "\nStill failing? Use a static IP (see the install docs)."
		return NameStatus{OK: false, Message: full + " isn't resolving yet", How: how}
	}
}

// renameHow is the legacy per-OS guidance for the opt-in "rename" mode.
func renameHow(name, full string) NameStatus {
	switch runtime.GOOS {
	case "darwin":
		return NameStatus{OK: false,
			Message: full + " not set yet",
			How:     "In Terminal: sudo scutil --set LocalHostName " + name + "  - or System Settings -> General -> Sharing -> Local hostname -> " + name + ". Then re-check."}
	case "linux":
		return NameStatus{OK: false,
			Message: full + " not set yet",
			How:     "In Terminal: sudo hostnamectl set-hostname " + name + " && sudo systemctl enable --now avahi-daemon. Then re-check."}
	case "windows":
		return NameStatus{OK: false,
			Message: full + " doesn't resolve yet",
			How: "Open PowerShell as Administrator and run these (the last one reboots):\n" +
				"  Set-NetConnectionProfile -NetworkCategory Private\n" +
				"  New-NetFirewallRule -DisplayName \"mDNS\" -Direction Inbound -Protocol UDP -LocalPort 5353 -Action Allow -Profile Private\n" +
				"  Rename-Computer -NewName \"" + name + "\" -Force -Restart\n" +
				"After the reboot, click Check. Still failing? Install Apple Bonjour, or use a static IP. (See docs/install-windows.md.)"}
	}
	return NameStatus{OK: false, Message: "unsupported OS"}
}
