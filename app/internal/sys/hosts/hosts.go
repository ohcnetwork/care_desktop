// Package hosts manages the loopback entry that lets the server's own browser
// open the clinic. See docs/architecture.md#mdns-advertising-and-self-heal.
package hosts

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/elevate"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

// Points <name>.local at loopback so the server's OWN browser can open the clinic;
// no host resolves a name its own second mDNS responder advertises. See
// docs/architecture.md#mdns-advertising-and-self-heal.

const marker = "# care-desktop"

func path() string {
	if runtime.GOOS != "windows" {
		return "/etc/hosts"
	}
	win := os.Getenv("WINDIR")
	if win == "" {
		win = `C:\Windows`
	}
	return filepath.Join(win, "System32", "drivers", "etc", "hosts")
}

func hasEntry(data, host string) bool {
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

func line(host string) string { return "127.0.0.1 " + host + " " + marker }

// Leading blank line terminates a hosts file with no trailing newline. echo, not
// printf: printf's "\n" would also be an AppleScript escape under `do shell script`.
func addSh(host string) string {
	p := path()
	return "echo '' >> " + p + "; echo " + elevate.ShQuote(line(host)) + " >> " + p
}

func addPS(host string) string {
	return `Add-Content -LiteralPath "$env:WINDIR\System32\drivers\etc\hosts" -Value ` +
		elevate.PSQuote(line(host))
}

// Unprivileged first: succeeds as root, so no prompt where none is needed.
func Step(log func(string), host string) (elevate.Step, bool) {
	if data, err := os.ReadFile(path()); err == nil && hasEntry(string(data), host) {
		return elevate.Step{}, false
	}
	if err := addUnprivileged(host); err == nil {
		logln(log, "Added a hosts entry so https://"+host+"/ opens on this computer.")
		return elevate.Step{}, false
	}
	return elevate.Step{
		What: "add " + host + " to this computer's hosts file, so the name works here",
		Sh:   addSh(host),
		PS:   addPS(host),
	}, true
}

func addUnprivileged(host string) error {
	if runtime.GOOS == "windows" {
		return proc.Command("powershell", "-NoProfile", "-Command", addPS(host)).Run()
	}
	return proc.Command("sh", "-c", addSh(host)).Run()
}

// removeHostsEntry drops the line install added. It returns a description of what
// was left behind, or "" when the file is clean — a surviving "127.0.0.1
// care.local" is silently poisonous: the machine keeps resolving the name to
// itself long after CARE is gone, and every other device on the LAN looks broken.
func Remove(log func(string), confirm func(string, string) bool, host string) string {
	data, err := os.ReadFile(path())
	if err != nil || !strings.Contains(string(data), marker) {
		return ""
	}
	logln(log, "Removing the "+host+" hosts entry...")
	if runtime.GOOS == "windows" {
		inner := `$p="$env:WINDIR\System32\drivers\etc\hosts"; (Get-Content -LiteralPath $p) | ` +
			`Where-Object { $_ -notmatch '` + marker + `' } | Set-Content -LiteralPath $p`
		ps := "Start-Process powershell -Verb RunAs -Wait -ArgumentList '-NoProfile','-Command'," +
			elevate.PSQuote(inner)
		_ = proc.Command("powershell", "-NoProfile", "-Command", ps).Run()
		return Leftover(host)
	}
	// cat back rather than mv: keeps the file's inode, owner, and mode.
	p := path()
	sh := `t=$(mktemp) && grep -v ` + elevate.ShQuote(marker) + ` ` + p +
		` > "$t" && cat "$t" > ` + p + `; rm -f "$t"`
	if elevate.Run(sh, false) == nil {
		return Leftover(host)
	}
	if confirm == nil || confirm("Remove the "+host+" hosts entry?",
		"Remove the line CARE added to this computer's hosts file?\n\nThis needs administrator approval.") {
		_ = elevate.Run(sh, true)
	}
	return Leftover(host)
}

// hostsEntryLeftover re-reads the file: the removal runs through a shell (and on
// Windows through an elevation prompt the operator can dismiss), so its exit
// status says little about whether the line is actually gone.
func Leftover(host string) string {
	data, err := os.ReadFile(path())
	if err != nil || !strings.Contains(string(data), marker) {
		return ""
	}
	return "The line \"" + line(host) + "\" is still in " + path() +
		". Until it is removed this computer resolves " + host + " to itself."
}

func logln(log func(string), s string) {
	if log != nil {
		log(s)
	}
}

// Present reports whether the hosts file still carries the line install added.
// Matched on the marker, not on a host name, so it finds the entry left by an
// install that used a different clinic address.
func Present() bool {
	data, err := os.ReadFile(path())
	return err == nil && strings.Contains(string(data), marker)
}
