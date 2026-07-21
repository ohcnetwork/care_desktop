package care

import (
	"fmt"
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

// DockerCheck is the one prerequisite the app can't bundle. Any Docker-compatible
// engine works (Docker Engine, Colima, Podman, Rancher Desktop, OrbStack, Docker
// Desktop) — we just need `docker` + the `docker compose` v2 plugin reachable.
func (e *Engine) DockerCheck() DockerStatus {
	cmd := exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	cmd.Env = e.baseEnv()
	out, err := cmd.Output()
	switch {
	case err == nil:
		if !e.hasCompose() {
			return DockerStatus{OK: false, Message: "Docker is running, but the Compose plugin is missing — install 'docker compose' (v2)."}
		}
		return DockerStatus{OK: true, Message: "Docker " + strings.TrimSpace(string(out))}
	case isNotFound(err):
		return DockerStatus{OK: false, Message: "Docker not found — install Docker (Docker Engine, Colima, or Docker Desktop) and start it."}
	default:
		return DockerStatus{OK: false, Message: "Docker is installed but not running — start it (Docker Desktop, Colima, …)."}
	}
}

// hasCompose reports whether the `docker compose` v2 plugin is available — often
// missing on non-Desktop installs, and the stack can't come up without it.
func (e *Engine) hasCompose() bool {
	cmd := exec.Command("docker", "compose", "version")
	cmd.Env = e.baseEnv()
	return cmd.Run() == nil
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

// WaitHealthy blocks until the stack actually answers healthy on :80 (Caddy →
// backend /ping/), or the timeout elapses. `docker compose up -d` only means the
// containers were *created* — the app server, Caddy, and its upstreams still need
// to come up before anything is really serving. Callers gate their success
// message (and the installer's "complete") on this so it's only reported once
// http://<name>.local is genuinely reachable.
func (e *Engine) WaitHealthy(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	last := Health{Detail: "nothing answering on :80"}
	for n := 1; ; n++ {
		if last = e.Ping(); last.Active {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("CARE did not become healthy within %s (%s)", timeout, last.Detail)
		}
		e.logln(fmt.Sprintf("  waiting for CARE to answer... (%d) — %s", n, last.Detail))
		time.Sleep(3 * time.Second)
	}
}

// EnsurePortFree fails fast when something other than our own stack already holds
// the app's host port (80). Without this, `docker compose up` fails deep inside with
// a cryptic "Bind for 0.0.0.0:80 failed: port is already allocated"; here we catch it
// first and return a clear, actionable message the installer shows on its failed
// screen. Our own running caddy is not a conflict (idempotent restarts must pass).
func (e *Engine) EnsurePortFree() error {
	if e.caddyRunning() {
		return nil // the listener on :80 is our own caddy
	}
	if !tcpBusy("127.0.0.1:80") {
		return nil
	}
	who := portOccupant(80)
	return fmt.Errorf("port 80 is already in use%s.\n"+
		"CARE serves the clinic app on port 80 (http://%s.local/). "+
		"Quit whatever is using port 80, then try again.", who, e.mdnsName())
}

// caddyRunning reports whether our compose stack's caddy service is already up, so a
// listener on :80 is ours (not a foreign conflict).
func (e *Engine) caddyRunning() bool {
	out, err := e.capture("docker", "compose", "ps", "--services", "--filter", "status=running")
	if err != nil {
		return false
	}
	for _, s := range strings.Split(out, "\n") {
		if strings.TrimSpace(s) == "caddy" {
			return true
		}
	}
	return false
}

// tcpBusy reports whether something is already listening at addr (a successful
// connect means the port is taken). Unprivileged and cross-platform — unlike trying
// to bind :80, which a non-root GUI app can't do even when the port is free.
func tcpBusy(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 700*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// portOccupant returns " by '<name>'" naming the process holding the port, or "" if
// it can't tell. Best-effort — the message reads fine without it.
func portOccupant(port int) string {
	var name string
	switch runtime.GOOS {
	case "darwin", "linux":
		out, err := exec.Command("lsof", "-nP", fmt.Sprintf("-iTCP:%d", port), "-sTCP:LISTEN", "-F", "c").Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if strings.HasPrefix(line, "c") { // "c<command>" field
					name = strings.TrimPrefix(line, "c")
					break
				}
			}
		}
	case "windows":
		// netstat -ano yields only a PID; mapping it to a name is extra work — skip.
	}
	if name == "" {
		return ""
	}
	return " by '" + name + "'"
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
