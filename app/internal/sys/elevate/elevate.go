// Package elevate runs shell fragments with administrator privileges and quotes
// strings safely for the shells involved. See docs/architecture.md.
package elevate

import (
	"runtime"
	"strings"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

// Step is one administrator-requiring action, expressed for both shell families.
type Step struct {
	What string // shown in the confirmation prompt
	Sh   string // POSIX (macOS, Linux)
	PS   string // PowerShell (Windows)
}

// PSQuote quotes a string as a PowerShell single-quoted literal.
func PSQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// ShQuote quotes a string as a POSIX single-quoted literal.
func ShQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// OSAQuote quotes a string as an AppleScript double-quoted literal.
func OSAQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// Run executes a POSIX shell fragment, elevating if asked. Windows callers use
// Steps instead, which goes through PowerShell's RunAs.
func Run(sh string, elevated bool) error {
	switch {
	case !elevated:
		return proc.Command("sh", "-c", sh).Run()
	case runtime.GOOS == "darwin":
		return proc.Command("osascript", "-e",
			"do shell script "+OSAQuote(sh)+" with administrator privileges").Run()
	default:
		return proc.Command("pkexec", "sh", "-c", sh).Run() // polkit GUI prompt
	}
}

// Steps runs every step under a single elevation prompt.
//
// Steps are joined with "; " rather than "&&" on purpose: a failing first step
// must not skip the second.
func Steps(steps []Step) error {
	pick := func(s Step) string { return s.Sh }
	if runtime.GOOS == "windows" {
		pick = func(s Step) string { return s.PS }
	}
	parts := make([]string, 0, len(steps))
	for _, s := range steps {
		parts = append(parts, pick(s))
	}
	joined := strings.Join(parts, "; ")

	if runtime.GOOS == "windows" {
		ps := "Start-Process powershell -Verb RunAs -Wait -ArgumentList '-NoProfile','-Command'," +
			PSQuote(joined)
		return proc.Command("powershell", "-NoProfile", "-Command", ps).Run()
	}
	return Run(joined, true)
}
