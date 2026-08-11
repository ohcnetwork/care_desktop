package care

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Points <name>.local at loopback so the server's OWN browser can open the clinic;
// no host resolves a name its own second mDNS responder advertises. See
// docs/architecture.md#mdns-advertising-and-self-heal.

const hostsMarker = "# care-desktop"

func hostsPath() string {
	if runtime.GOOS != "windows" {
		return "/etc/hosts"
	}
	win := os.Getenv("WINDIR")
	if win == "" {
		win = `C:\Windows`
	}
	return filepath.Join(win, "System32", "drivers", "etc", "hosts")
}

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

func psSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func shSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func asQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

func hostsLine(host string) string { return "127.0.0.1 " + host + " " + hostsMarker }

// Leading blank line terminates a hosts file with no trailing newline. echo, not
// printf: printf's "\n" would also be an AppleScript escape under `do shell script`.
func hostsAddSh(host string) string {
	p := hostsPath()
	return "echo '' >> " + p + "; echo " + shSingleQuote(hostsLine(host)) + " >> " + p
}

func hostsAddPS(host string) string {
	return `Add-Content -LiteralPath "$env:WINDIR\System32\drivers\etc\hosts" -Value ` +
		psSingleQuote(hostsLine(host))
}

// Unprivileged first: succeeds as root, so no prompt where none is needed.
func (e *Engine) hostsStep(host string) (privilegedStep, bool) {
	if data, err := os.ReadFile(hostsPath()); err == nil && hostsHasEntry(string(data), host) {
		return privilegedStep{}, false
	}
	if err := e.addHostsUnprivileged(host); err == nil {
		e.logln("Added a hosts entry so https://" + host + "/ opens on this computer.")
		return privilegedStep{}, false
	}
	return privilegedStep{
		what: "add " + host + " to this computer's hosts file, so the name works here",
		sh:   hostsAddSh(host),
		ps:   hostsAddPS(host),
	}, true
}

func (e *Engine) addHostsUnprivileged(host string) error {
	if runtime.GOOS == "windows" {
		return newCmd("powershell", "-NoProfile", "-Command", hostsAddPS(host)).Run()
	}
	return newCmd("sh", "-c", hostsAddSh(host)).Run()
}

func (e *Engine) removeHostsEntry() {
	data, err := os.ReadFile(hostsPath())
	if err != nil || !strings.Contains(string(data), hostsMarker) {
		return
	}
	e.logln("Removing the " + e.host() + " hosts entry...")
	if runtime.GOOS == "windows" {
		inner := `$p="$env:WINDIR\System32\drivers\etc\hosts"; (Get-Content -LiteralPath $p) | ` +
			`Where-Object { $_ -notmatch '` + hostsMarker + `' } | Set-Content -LiteralPath $p`
		ps := "Start-Process powershell -Verb RunAs -Wait -ArgumentList '-NoProfile','-Command'," +
			psSingleQuote(inner)
		_ = newCmd("powershell", "-NoProfile", "-Command", ps).Run()
		return
	}
	// cat back rather than mv: keeps the file's inode, owner, and mode.
	p := hostsPath()
	sh := `t=$(mktemp) && grep -v ` + shSingleQuote(hostsMarker) + ` ` + p +
		` > "$t" && cat "$t" > ` + p + `; rm -f "$t"`
	if e.runPrivileged(sh, false) == nil {
		return
	}
	if e.Confirm == nil || e.Confirm("Remove the "+e.host()+" hosts entry?",
		"Remove the line CARE added to this computer's hosts file?\n\nThis needs administrator approval.") {
		_ = e.runPrivileged(sh, true)
	}
}

func (e *Engine) runPrivileged(sh string, elevated bool) error {
	switch {
	case !elevated:
		return newCmd("sh", "-c", sh).Run()
	case runtime.GOOS == "darwin":
		return newCmd("osascript", "-e",
			"do shell script "+asQuote(sh)+" with administrator privileges").Run()
	default:
		return newCmd("pkexec", "sh", "-c", sh).Run() // polkit GUI prompt
	}
}
