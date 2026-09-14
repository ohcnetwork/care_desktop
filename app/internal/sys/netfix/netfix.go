// Package netfix reads and repairs the Windows network profile and firewall
// rules that let other LAN devices reach the clinic. See docs/architecture.md.
package netfix

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/elevate"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

// fwPrefix tags the firewall rules we create so uninstall can remove exactly ours.
const fwPrefix = "CARE Desktop "

// Status reports whether the active network lets other LAN devices reach the
// clinic. Applicable is false off Windows (the wizard hides the row); Fixable means
// FixNetwork can repair it in one elevated click.
type Status struct {
	Applicable bool   `json:"applicable"`
	OK         bool   `json:"ok"`
	Message    string `json:"message"`
	How        string `json:"how"`
	Fixable    bool   `json:"fixable"`
}

func Check(run proc.Runner) Status {
	if runtime.GOOS != "windows" {
		return Status{Applicable: false, OK: true, Message: "not needed on this OS"}
	}
	out, err := run.Capture("powershell", "-NoProfile", "-Command",
		"Get-NetConnectionProfile | ForEach-Object { $_.NetworkCategory }")
	if err != nil {
		return Status{Applicable: true, OK: true, Message: "couldn't read the network profile"}
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.EqualFold(strings.TrimSpace(line), "Public") {
			return Status{
				Applicable: true, OK: false, Fixable: true,
				Message: "other devices can't reach this computer",
				How: "Windows is treating this WiFi as public, so it hides this computer " +
					"from the tablets and phones in your clinic. Click Fix to allow them in.",
			}
		}
	}
	if !RulesPresent(run) {
		return Status{
			Applicable: true, OK: false, Fixable: true,
			Message: "the clinic's ports aren't open",
			How: "Windows blocks incoming connections unless a rule allows them. " +
				"Click Fix to open the clinic's ports.",
		}
	}
	return Status{Applicable: true, OK: true, Message: "other devices can reach this computer"}
}

func Fix(log func(string)) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	inner := strings.Join([]string{
		`Get-NetConnectionProfile | Where-Object { $_.NetworkCategory -eq 'Public' } | Set-NetConnectionProfile -NetworkCategory Private`,
		ensureRule("mDNS", "UDP", 5353),
		ensureRule("HTTPS", "TCP", 443),
		ensureRule("HTTP", "TCP", 80),
	}, "; ")
	ps := "$p = Start-Process powershell -Verb RunAs -Wait -PassThru -ArgumentList '-NoProfile','-Command'," +
		elevate.PSQuote(inner) + "; exit $p.ExitCode"
	logln(log, "This network is protected. Updating it so other devices can reach the clinic (approve the prompt)...")
	if err := proc.Command("powershell", "-NoProfile", "-Command", ps).Run(); err != nil {
		return fmt.Errorf("couldn't update the network settings (prompt may have been declined): %w", err)
	}
	return nil
}

// ensureRule adds one inbound allow rule if a rule of that name doesn't already exist.
func ensureRule(label, proto string, port int) string {
	name := fwPrefix + label
	return fmt.Sprintf(
		`if (-not (Get-NetFirewallRule -DisplayName '%s' -ErrorAction SilentlyContinue)) { `+
			`New-NetFirewallRule -DisplayName '%s' -Direction Inbound -Protocol %s -LocalPort %d -Action Allow -Profile Private,Domain | Out-Null }`,
		name, name, proto, port)
}

func Undo(log func(string)) {
	if runtime.GOOS != "windows" {
		return
	}
	inner := `Get-NetFirewallRule -DisplayName '` + fwPrefix + `*' -ErrorAction SilentlyContinue | Remove-NetFirewallRule`
	ps := "Start-Process powershell -Verb RunAs -Wait -ArgumentList '-NoProfile','-Command'," + elevate.PSQuote(inner)
	logln(log, "Removing the clinic's firewall rules (approve the prompt)...")
	_ = proc.Command("powershell", "-NoProfile", "-Command", ps).Run()
}

func logln(log func(string), s string) {
	if log != nil {
		log(s)
	}
}

func RulesPresent(run proc.Runner) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	out, err := run.Capture("powershell", "-NoProfile", "-Command",
		"(Get-NetFirewallRule -DisplayName '"+fwPrefix+"*' -ErrorAction SilentlyContinue | Measure-Object).Count")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) != "" && strings.TrimSpace(out) != "0"
}
