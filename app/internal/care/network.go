package care

import (
	"fmt"
	"runtime"
	"strings"
)

// fwPrefix tags the firewall rules we create so uninstall can remove exactly ours.
const fwPrefix = "CARE Desktop "

// NetworkStatus reports whether the active network lets other LAN devices reach the
// clinic. Applicable is false off Windows (the wizard hides the row); Fixable means
// FixNetwork can repair it in one elevated click.
type NetworkStatus struct {
	Applicable bool   `json:"applicable"`
	OK         bool   `json:"ok"`
	Message    string `json:"message"`
	How        string `json:"how"`
	Fixable    bool   `json:"fixable"`
}

// NetworkCheck flags a Public Windows profile - the usual reason other devices can't
// find care.local (Windows suppresses mDNS discovery on Public).
func (e *Engine) NetworkCheck() NetworkStatus {
	if runtime.GOOS != "windows" {
		return NetworkStatus{Applicable: false, OK: true, Message: "not needed on this OS"}
	}
	out, err := e.capture("powershell", "-NoProfile", "-Command",
		"Get-NetConnectionProfile | ForEach-Object { $_.NetworkCategory }")
	if err != nil {
		return NetworkStatus{Applicable: true, OK: true, Message: "couldn't read the network profile"}
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.EqualFold(strings.TrimSpace(line), "Public") {
			return NetworkStatus{
				Applicable: true, OK: false, Fixable: true,
				Message: "this Wi-Fi is set to Public",
				How: "Windows hides this server from other devices on a Public network. " +
					"Click Fix to set it to Private and open the clinic's ports (mDNS 5353, HTTPS 443, HTTP 80).",
			}
		}
	}
	return NetworkStatus{Applicable: true, OK: true, Message: "network is Private"}
}

// FixNetwork sets Public profiles to Private and opens the clinic's inbound ports, via
// one elevated (UAC) PowerShell. Idempotent, Windows-only.
func (e *Engine) FixNetwork() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	inner := strings.Join([]string{
		`Get-NetConnectionProfile | Where-Object { $_.NetworkCategory -eq 'Public' } | Set-NetConnectionProfile -NetworkCategory Private`,
		ensureRule("mDNS", "UDP", 5353),
		ensureRule("HTTPS", "TCP", 443),
		ensureRule("HTTP", "TCP", 80),
	}, "; ")
	ps := "Start-Process powershell -Verb RunAs -Wait -ArgumentList '-NoProfile','-Command'," + psSingleQuote(inner)
	e.logln("Setting this network to Private and opening the clinic's ports (approve the prompt)...")
	if err := newCmd("powershell", "-NoProfile", "-Command", ps).Run(); err != nil {
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

// undoNetworkChanges reverses FixNetwork (Private->Public + drop our rules) in one
// elevated prompt. Called on uninstall; best-effort, Windows-only.
func (e *Engine) undoNetworkChanges() {
	if runtime.GOOS != "windows" {
		return
	}
	inner := strings.Join([]string{
		`Get-NetConnectionProfile | Where-Object { $_.NetworkCategory -eq 'Private' } | Set-NetConnectionProfile -NetworkCategory Public`,
		`Get-NetFirewallRule -DisplayName '` + fwPrefix + `*' -ErrorAction SilentlyContinue | Remove-NetFirewallRule`,
	}, "; ")
	ps := "Start-Process powershell -Verb RunAs -Wait -ArgumentList '-NoProfile','-Command'," + psSingleQuote(inner)
	e.logln("Reverting the network to Public and removing the firewall rules (approve the prompt)...")
	_ = newCmd("powershell", "-NoProfile", "-Command", ps).Run()
}
