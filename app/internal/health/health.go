// Package health probes whether the clinic is answering and whether the ports
// it needs are free. See docs/architecture.md.
package health

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

// Health is the app-facing view of whether the clinic answers.
type Health struct {
	Active bool   `json:"active"`
	Code   int    `json:"code"`
	Detail string `json:"detail"`
}

// Ping hits https://localhost/ping/ with a short timeout. Deliberately localhost
// rather than the clinic's name: this asks "is Caddy answering on this machine",
// which must not also depend on mDNS resolving <name>.local - that is a separate
// failure with its own check. The cert is Caddy's
// self-signed internal CA, so verification is skipped - this is a same-host
// liveness probe, not a trust decision.
func Ping() Health {
	client := &http.Client{
		Timeout:   3 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, // ponytail: local self-signed cert, host probe only
	}
	resp, err := client.Get("https://localhost/ping/")
	if err != nil {
		return Health{Active: false, Code: 0, Detail: "nothing answering on :443"}
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return Health{Active: true, Code: 200, Detail: "healthy"}
	}
	return Health{Active: false, Code: resp.StatusCode, Detail: "HTTP " + strings.TrimSpace(resp.Status)}
}

// WaitHealthy blocks until the stack actually answers healthy on :443 (Caddy ->
// backend /ping/), or the timeout elapses. `docker compose up -d` only means the
// containers were *created* - the app server, Caddy, and its upstreams still need
// to come up before anything is really serving. Callers gate their success
// message (and the installer's "complete") on this so it's only reported once
// https://<name>.local is genuinely reachable.
func Wait(log func(string), host string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	last := Health{Detail: "nothing answering yet"}
	for n := 1; ; n++ {
		if last = Ping(); last.Active {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("CARE did not become healthy within %s (%s)", timeout, last.Detail)
		}
		logln(log, fmt.Sprintf("  waiting for CARE to answer... (%d) - %s", n, last.Detail))
		time.Sleep(3 * time.Second)
	}
}

// EnsurePortFree fails fast when something other than our own stack already holds
// the app's host ports (80 redirect + 443 https). Without this, `docker compose up`
// fails deep inside with a cryptic "Bind for 0.0.0.0:443 failed: port is already
// allocated"; here we catch it first and return a clear, actionable message the
// installer shows on its failed screen. Our own running caddy is not a conflict
// (idempotent restarts must pass).
func EnsurePortFree(run proc.Runner, host string) error {
	if caddyRunning(run) {
		return nil // the listeners on :80/:443 are our own caddy
	}
	for _, port := range []int{80, 443} {
		if !tcpBusy(fmt.Sprintf("127.0.0.1:%d", port)) {
			continue
		}
		who := portOccupant(port)
		return fmt.Errorf("port %d is already in use%s.\n"+
			"CARE serves the clinic app on https://%s.local/ (ports 80 and 443). "+
			"Quit whatever is using port %d, then try again.", port, who, strings.TrimSuffix(host, ".local"), port)
	}
	return nil
}

// caddyRunning reports whether our compose stack's caddy service is already up, so a
// listener on :80 is ours (not a foreign conflict).
func caddyRunning(run proc.Runner) bool {
	out, err := run.Capture("docker", "compose", "ps", "--services", "--filter", "status=running")
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
		out, err := proc.Command("lsof", "-nP", fmt.Sprintf("-iTCP:%d", port), "-sTCP:LISTEN", "-F", "c").Output()
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

func logln(log func(string), s string) {
	if log != nil {
		log(s)
	}
}
