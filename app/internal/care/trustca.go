package care

import (
	"crypto/sha1"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// trustLocalCA installs Caddy's internal root into the *host* trust store, so a
// browser on the server machine opens https://care.local without warnings.
// It tries without elevation first; if that needs admin it asks the user
// (native yes/no) and re-runs through the OS elevation prompt (osascript /
// UAC / pkexec). Best-effort and idempotent: skipped once the host trusts us,
// so Start never re-prompts or fails.
func (e *Engine) trustLocalCA() {
	host := e.mdnsName() + ".local"
	if e.hostTrustsCARE(host) {
		return // already trusted - don't re-run or re-prompt
	}
	pem, err := e.capture("docker", "compose", "exec", "-T", "caddy",
		"cat", "/data/caddy/pki/authorities/local/root.crt")
	if err != nil || !strings.Contains(pem, "BEGIN CERTIFICATE") {
		return // caddy not ready or no root yet - silently skip
	}
	f, err := os.CreateTemp("", "care-root-*.crt")
	if err != nil {
		return
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(pem); err != nil {
		f.Close()
		return
	}
	f.Close()

	// First try without elevation.
	if err := e.installCA(f.Name(), false); err == nil {
		e.logln("This machine now trusts https://" + host + "/.")
		return
	}
	// Needs admin: ask, then re-run through the OS elevation prompt.
	if e.Confirm == nil || !e.Confirm("Trust CARE on this machine?",
		"Add the CARE security certificate to this computer so browsers open "+
			"https://"+host+" without warnings?\n\nThis needs administrator approval.") {
		e.logln("Skipped trusting the certificate here. Open http://" + host + "/setup to do it later.")
		return
	}
	if err := e.installCA(f.Name(), true); err != nil {
		e.logln("Could not trust the certificate (" + err.Error() +
			"). Open http://" + host + "/setup to install it.")
		return
	}
	e.logln("This machine now trusts https://" + host + "/.")
}

// installCA adds the cert at path to the host trust store. elevated=false uses
// the current user's rights (may still pop a per-OS auth); elevated=true wraps
// the command in the platform's privilege-escalation prompt.
func (e *Engine) installCA(path string, elevated bool) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		if elevated {
			// System keychain via the native admin prompt.
			script := fmt.Sprintf(
				`do shell script "security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain \"%s\"" with administrator privileges`,
				path)
			cmd = newCmd("osascript", "-e", script)
		} else {
			// login keychain = current user, no sudo.
			cmd = newCmd("security", "add-trusted-cert", "-r", "trustRoot",
				"-k", os.Getenv("HOME")+"/Library/Keychains/login.keychain-db", path)
		}
	case "windows":
		if elevated {
			// UAC prompt via RunAs; -Wait so we see the exit status.
			ps := fmt.Sprintf(
				`Start-Process certutil -ArgumentList '-addstore','-f','Root','%s' -Verb RunAs -Wait`,
				path)
			cmd = newCmd("powershell", "-NoProfile", "-Command", ps)
		} else {
			cmd = newCmd("certutil", "-addstore", "-f", "Root", path)
		}
	case "linux":
		// Copy to the CA anchors then refresh (Debian layout first, then RHEL).
		sh := "cp " + path + " /usr/local/share/ca-certificates/care-root.crt && update-ca-certificates " +
			"|| { cp " + path + " /etc/pki/ca-trust/source/anchors/care-root.crt && update-ca-trust; }"
		if elevated {
			cmd = newCmd("pkexec", "sh", "-c", sh) // polkit GUI prompt
		} else {
			cmd = newCmd("sh", "-c", sh)
		}
	default:
		return fmt.Errorf("unsupported OS %s", runtime.GOOS)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// hostTrustsCARE reports whether the host's own trust store already accepts the
// live https cert - the idempotency gate for trustLocalCA (no InsecureSkipVerify
// here on purpose: we want the real system verdict).
func (e *Engine) hostTrustsCARE(host string) bool {
	c := &http.Client{Timeout: 4 * time.Second}
	resp, err := c.Get("https://" + host + "/ping/")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// caddyRootPEM reads Caddy's internal root CA out of the running container. Must
// be called BEFORE `compose down -v` - the root lives in the caddy-data volume,
// which teardown destroys. Empty string if caddy isn't up (nothing to untrust).
func (e *Engine) caddyRootPEM() string {
	pem, err := e.capture("docker", "compose", "exec", "-T", "caddy",
		"cat", "/data/caddy/pki/authorities/local/root.crt")
	if err != nil || !strings.Contains(pem, "BEGIN CERTIFICATE") {
		return ""
	}
	return pem
}

// untrustLocalCA removes the CARE root from the host trust store on uninstall -
// the mirror of trustLocalCA. Best-effort and never fatal: it deletes exactly the
// cert we installed (matched by SHA-1 fingerprint), so it can't touch anything
// else. pem is the root captured before teardown; empty pem = nothing to do.
func (e *Engine) untrustLocalCA(pem string) {
	fp := certSHA1Hex(pem)
	if fp == "" {
		return
	}
	switch runtime.GOOS {
	case "darwin":
		// User keychain first (no prompt). delete-certificate takes the 40-hex SHA-1.
		_ = newCmd("security", "delete-certificate", "-Z", fp,
			os.Getenv("HOME")+"/Library/Keychains/login.keychain-db").Run()
		// System keychain needs admin - only prompt if the cert is actually there.
		if e.certInSystemKeychain(fp) {
			if e.Confirm == nil || e.Confirm("Remove CARE certificate?",
				"Remove the CARE security certificate from this computer's System keychain?\n\nThis needs administrator approval.") {
				script := fmt.Sprintf(
					`do shell script "security delete-certificate -Z %s /Library/Keychains/System.keychain" with administrator privileges`, fp)
				_ = newCmd("osascript", "-e", script).Run()
			}
		}
	case "windows":
		ps := fmt.Sprintf(
			`Start-Process certutil -ArgumentList '-delstore','Root','%s' -Verb RunAs -Wait`, fp)
		_ = newCmd("powershell", "-NoProfile", "-Command", ps).Run()
	case "linux":
		// We wrote these exact files in installCA; remove them and refresh.
		sh := "rm -f /usr/local/share/ca-certificates/care-root.crt /etc/pki/ca-trust/source/anchors/care-root.crt; " +
			"update-ca-certificates 2>/dev/null; update-ca-trust 2>/dev/null; true"
		if newCmd("sh", "-c", sh).Run() != nil && e.Confirm != nil &&
			e.Confirm("Remove CARE certificate?", "Remove the CARE security certificate from this computer? Needs administrator approval.") {
			_ = newCmd("pkexec", "sh", "-c", sh).Run()
		}
	default:
		return
	}
	e.logln("Removed the CARE certificate from this machine's trust store.")
}

// certSHA1Hex returns the uppercase hex SHA-1 of the first certificate in pem -
// the fingerprint macOS `security` and Windows `certutil` use to identify a cert.
// Returns "" for anything that isn't a parseable certificate.
func certSHA1Hex(pemData string) string {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil || block.Type != "CERTIFICATE" {
		return ""
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return "" // a CERTIFICATE block with junk DER - not something we installed
	}
	sum := sha1.Sum(block.Bytes)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

// certInSystemKeychain reports whether a cert with this SHA-1 is in the macOS
// System keychain (reading it needs no admin - only deleting does), so we avoid a
// pointless admin prompt when the cert was only ever added to the login keychain.
func (e *Engine) certInSystemKeychain(fp string) bool {
	out, err := newCmd("security", "find-certificate", "-a", "-Z",
		"/Library/Keychains/System.keychain").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToUpper(string(out)), fp)
}
