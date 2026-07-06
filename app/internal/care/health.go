package care

import (
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// DockerStatus reports whether the Docker daemon is reachable.
type DockerStatus struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// DockerCheck is the one prerequisite the app can't bundle.
func (e *Engine) DockerCheck() DockerStatus {
	cmd := exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	cmd.Env = e.baseEnv()
	out, err := cmd.Output()
	switch {
	case err == nil:
		return DockerStatus{OK: true, Message: "Docker " + strings.TrimSpace(string(out))}
	case isNotFound(err):
		return DockerStatus{OK: false, Message: "Docker not found — install Docker Desktop to continue."}
	default:
		return DockerStatus{OK: false, Message: "Docker is installed but not running — start Docker Desktop."}
	}
}

func isNotFound(err error) bool {
	return strings.Contains(err.Error(), "executable file not found") ||
		strings.Contains(err.Error(), "cannot find the file")
}

// Health reports whether the app answers on :80 (through Caddy → backend /ping/).
type Health struct {
	Active bool   `json:"active"`
	Code   int    `json:"code"`
	Detail string `json:"detail"`
}

// Ping hits http://localhost/ping/ with a short timeout.
func (e *Engine) Ping() Health {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://localhost/ping/")
	if err != nil {
		return Health{Active: false, Code: 0, Detail: "nothing answering on :80"}
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return Health{Active: true, Code: 200, Detail: "healthy"}
	}
	return Health{Active: false, Code: resp.StatusCode, Detail: "HTTP " + strings.TrimSpace(resp.Status)}
}

// GitCheck reports whether git is available (needed for the one-time clone+build).
func (e *Engine) GitCheck() DockerStatus {
	cmd := exec.Command("git", "--version")
	cmd.Env = e.baseEnv()
	out, err := cmd.Output()
	if err == nil {
		return DockerStatus{OK: true, Message: strings.TrimSpace(string(out))}
	}
	return DockerStatus{OK: false, Message: "Git not found — install Git (Git for Windows / Xcode CLT / apt-get git)."}
}

// NameStatus reports whether this machine is reachable as <name>.local, with a
// per-OS "how" the wizard shows when it isn't.
type NameStatus struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	How     string `json:"how"`
}

// MDNSCheck verifies that <name>.local actually resolves right now — a real
// functional test (does the LAN answer?), uniform across OSes. It's gated in the
// installer because the frontend is baked to http://care.local. The "how" text when
// it fails depends on the mDNS mode. Note: in "advertise" mode the app answers this
// itself, so the app's MDNSStatus reports green as soon as its responder is up.
func (e *Engine) MDNSCheck() NameStatus {
	name := e.mdnsName() // e.g. "care"
	full := name + ".local"
	if _, err := net.LookupHost(full); err == nil {
		return NameStatus{OK: true, Message: full + " resolves"}
	}
	switch e.MDNSMode() {
	case "rename":
		return renameHow(name, full)
	case "off":
		return NameStatus{OK: false,
			Message: full + " not advertised (mDNS is off)",
			How:     "You're on static-IP mode. Open http://<server-ip>/ instead of " + full + ", or set CARE_MDNS_MODE=advertise."}
	default: // advertise
		how := "Open (and keep open) the CARE Desktop app — it advertises " + full +
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
			How:     "In Terminal: sudo scutil --set LocalHostName " + name + "  — or System Settings → General → Sharing → Local hostname → " + name + ". Then re-check."}
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
