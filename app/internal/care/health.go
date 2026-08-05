package care

import (
	"context"
	"crypto/tls"
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
// Desktop) - we just need `docker` + the `docker compose` v2 plugin reachable.
func (e *Engine) DockerCheck() DockerStatus {
	cmd := exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	cmd.Env = e.baseEnv()
	out, err := cmd.Output()
	switch {
	case err == nil:
		if !e.hasCompose() {
			return DockerStatus{OK: false, Message: "Docker is running, but the Compose plugin is missing - install 'docker compose' (v2)."}
		}
		return DockerStatus{OK: true, Message: "Docker " + strings.TrimSpace(string(out))}
	case isNotFound(err):
		return DockerStatus{OK: false, Message: "Docker not found - install Docker (Docker Engine, Colima, or Docker Desktop) and start it."}
	default:
		return DockerStatus{OK: false, Message: "Docker is installed but not running - start it (Docker Desktop, Colima, ...)."}
	}
}

// hasCompose reports whether the `docker compose` v2 plugin is available - often
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

// Health reports whether the app answers through Caddy -> backend /ping/.
type Health struct {
	Active bool   `json:"active"`
	Code   int    `json:"code"`
	Detail string `json:"detail"`
}

// Ping asks the stack for /ping/ through Caddy, with a short timeout.
func (e *Engine) Ping() Health {
	if !e.Configured() {
		return Health{Active: false, Detail: "no clinic web address configured yet"}
	}
	client := e.pingClient()
	resp, err := client.Get("https://" + e.publicHost() + "/ping/")
	if err != nil {
		return Health{Active: false, Code: 0, Detail: "nothing answering on :443"}
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return Health{Active: true, Code: 200, Detail: "healthy"}
	}
	return Health{Active: false, Code: resp.StatusCode, Detail: "HTTP " + strings.TrimSpace(resp.Status)}
}

// pingClient builds the health probe's client. The URL keeps the public name (that's
// what sets TLS SNI and what the certificate is checked against) while this transport
// dials loopback - so the probe validates the certificate honestly without depending
// on clinic DNS, which may not answer for the name even when CARE is fine.
func (e *Engine) pingClient() *http.Client {
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, network, "127.0.0.1:443")
		},
	}
	// On a staging CA the certificate chains to an untrusted root by design, so a
	// verifying probe would report the stack as down when it is in fact working -
	// exactly during the test run where a clear result matters most. Skipping
	// verification is safe here and only here: we dial loopback, so there is no
	// network path for anything else to answer.
	if e.acmeCA() != acmeProduction {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &http.Client{Timeout: 5 * time.Second, Transport: tr}
}

// WaitHealthy blocks until the stack actually answers healthy (Caddy -> backend
// /ping/), or the timeout elapses. `docker compose up -d` only means the containers
// were *created* - the app server, Caddy, and its upstreams still need to come up
// before anything is really serving. Callers gate their success message (and the
// installer's "complete") on this so it's only reported once the clinic's address is
// genuinely reachable.
//
// With HTTPS on, the first start also waits on certificate issuance: Caddy has to
// complete the ACME DNS-01 exchange before it can serve anything, which adds up to a
// couple of minutes on top. That's why callers pass a generous timeout.
func (e *Engine) WaitHealthy(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	last := e.Ping()
	for n := 1; ; n++ {
		if last = e.Ping(); last.Active {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("CARE did not become healthy within %s (%s)", timeout, last.Detail)
		}
		e.logln(fmt.Sprintf("  waiting for CARE to answer... (%d) - %s", n, last.Detail))
		time.Sleep(3 * time.Second)
	}
}

// EnsurePortFree fails fast when something other than our own stack already holds
// :443. Without this, `docker compose up` fails deep inside with a cryptic "Bind for
// 0.0.0.0:443 failed: port is already allocated"; here we catch it first and return a
// clear, actionable message the installer shows on its failed screen. Our own running
// caddy is not a conflict (idempotent restarts must pass).
func (e *Engine) EnsurePortFree() error {
	if e.caddyRunning() {
		return nil // the listener is our own caddy
	}
	if !tcpBusy(fmt.Sprintf("127.0.0.1:%d", httpsPort)) {
		return nil
	}
	return fmt.Errorf("port %d is already in use%s.\n"+
		"CARE serves the clinic app on port %d. "+
		"Quit whatever is using it, then try again.",
		httpsPort, portOccupant(httpsPort), httpsPort)
}

// caddyRunning reports whether our compose stack's caddy service is already up, so a
// listener on :443 is ours (not a foreign conflict).
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
// connect means the port is taken). Unprivileged and cross-platform - unlike trying
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
// it can't tell. Best-effort - the message reads fine without it.
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
		// netstat -ano yields only a PID; mapping it to a name is extra work - skip.
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
	return DockerStatus{OK: false, Message: "Git not found - install Git (Git for Windows / Xcode CLT / apt-get git)."}
}

// NameStatus is a wizard-facing check result: did it pass, what to show, and how to
// fix it when it didn't. Used for the clinic's web-address check (see tls.go).
type NameStatus struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	How     string `json:"how"`
}
