package care

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Windows can't resolve "<name>.local" for its own browser without Bonjour, so
// the FE loads but every same-origin call it makes (the API, the plugin bundles)
// fails with ERR_NAME_NOT_RESOLVED. We fix that WITHOUT renaming the machine: a
// single hosts-file line mapping the name to loopback. Caddy already listens on
// 443 on all interfaces (incl. 127.0.0.1), so the server's own browser works.
// LAN clients still use mDNS/their own hosts entry - this is only for THIS PC.
//
// Windows-only: macOS (Bonjour) and Linux (avahi/nss-mdns) resolve .local for the
// local machine already.

const hostsMarker = "# care-desktop"

func hostsPath() string {
	win := os.Getenv("WINDIR")
	if win == "" {
		win = `C:\Windows`
	}
	return filepath.Join(win, "System32", "drivers", "etc", "hosts")
}

// hostsHasEntry reports whether an active (non-commented) line already resolves
// host - either one we added or one a user added by hand - so we never duplicate.
func hostsHasEntry(data, host string) bool {
	for _, ln := range strings.Split(data, "\n") {
		line := strings.TrimSpace(ln)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i] // drop any trailing comment
		}
		for _, field := range strings.Fields(line) {
			if strings.EqualFold(field, host) {
				return true
			}
		}
	}
	return false
}

// psSingleQuote wraps s as a PowerShell single-quoted literal (doubling any inner
// quote), safe to embed inside another single-quoted -ArgumentList element.
func psSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// ensureHostsEntry adds "127.0.0.1 <name>.local # care-desktop" to the Windows
// hosts file if absent, via an elevated (UAC) PowerShell - the same prompt the
// cert-trust step uses. Best-effort: a failure just prints the manual step.
func (e *Engine) ensureHostsEntry() {
	if runtime.GOOS != "windows" {
		return
	}
	host := e.mdnsName() + ".local"
	if data, err := os.ReadFile(hostsPath()); err == nil && hostsHasEntry(string(data), host) {
		return
	}
	line := "127.0.0.1 " + host + " " + hostsMarker
	inner := `Add-Content -LiteralPath "$env:WINDIR\System32\drivers\etc\hosts" -Value ` + psSingleQuote(line)
	ps := "Start-Process powershell -Verb RunAs -Wait -ArgumentList '-NoProfile','-Command'," + psSingleQuote(inner)
	e.logln("Adding a hosts entry so https://" + host + "/ resolves on this PC (approve the prompt)...")
	if err := newCmd("powershell", "-NoProfile", "-Command", ps).Run(); err != nil {
		e.logln("(could not add hosts entry - add '127.0.0.1 " + host + "' to " + hostsPath() + " by hand)")
	}
}

// removeHostsEntry strips the lines we added (matched by marker) on uninstall,
// elevated. Only prompts if one of our lines is actually present.
func (e *Engine) removeHostsEntry() {
	if runtime.GOOS != "windows" {
		return
	}
	data, err := os.ReadFile(hostsPath())
	if err != nil || !strings.Contains(string(data), hostsMarker) {
		return
	}
	inner := `$p="$env:WINDIR\System32\drivers\etc\hosts"; (Get-Content -LiteralPath $p) | ` +
		`Where-Object { $_ -notmatch '` + hostsMarker + `' } | Set-Content -LiteralPath $p`
	ps := "Start-Process powershell -Verb RunAs -Wait -ArgumentList '-NoProfile','-Command'," + psSingleQuote(inner)
	e.logln("Removing the care.local hosts entry (approve the prompt)...")
	_ = newCmd("powershell", "-NoProfile", "-Command", ps).Run()
}
