package clinic

import (
	"os/exec"
	"runtime"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/hostname"
)

// ensureMDNS makes http://<name>.local resolve on the LAN by renaming the machine.
// Only runs in "rename" mode now - the default "advertise" mode uses the in-app
// pure-Go responder (Advertise) instead, which needs no rename. Per-OS, best-effort:
// failures never abort setup (naming can be fixed by hand / static IP).
func (e *Clinic) ensureMDNS() {
	if e.MDNSMode() != "rename" {
		return
	}
	name := e.mdnsName()
	switch runtime.GOOS {
	case "darwin":
		cur, _ := e.capture("scutil", "--get", "LocalHostName")
		if cur == name {
			return
		}
		hostname.SavePrevious(e.InstallDir, e.mdnsName(), cur)
		e.logln("Naming this Mac '" + name + "' so devices can use http://" + name + ".local ...")
		if err := e.run(nil, "sudo", "scutil", "--set", "LocalHostName", name); err != nil {
			e.logln("(skipped renaming - use the server IP)")
		}
	case "linux":
		if _, err := exec.LookPath("avahi-daemon"); err != nil {
			_ = e.run(nil, "sh", "-c", "sudo apt-get install -y avahi-daemon || sudo dnf install -y avahi || true")
		}
		cur, _ := e.capture("hostnamectl", "--static")
		hostname.SavePrevious(e.InstallDir, e.mdnsName(), cur)
		_ = e.run(nil, "sudo", "hostnamectl", "set-hostname", name)
		_ = e.run(nil, "sudo", "systemctl", "enable", "--now", "avahi-daemon")
	case "windows":
		// This PC resolves <name>.local via the hosts entry (ensureLocalAccess).
		e.logln("Windows: this PC uses a hosts entry for https://" + name + ".local; other devices use mDNS or a static IP.")
	}
}
