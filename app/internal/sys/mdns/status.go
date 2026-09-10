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
// installer because the frontend is baked to http://care.local. The app answers
// this itself via Advertise, so status goes green as soon as its responder is up.
// name is the bare label (e.g. "care").
func Check(name string) NameStatus {
	full := name + ".local"
	if _, err := net.LookupHost(full); err == nil {
		return NameStatus{OK: true, Message: full + " resolves"}
	}
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
