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

func caInstallSh(path string) string {
	if runtime.GOOS == "darwin" {
		return "security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain " +
			shSingleQuote(path)
	}
	// Debian layout first, then RHEL.
	q := shSingleQuote(path)
	return "cp " + q + " /usr/local/share/ca-certificates/care-root.crt && update-ca-certificates " +
		"|| { cp " + q + " /etc/pki/ca-trust/source/anchors/care-root.crt && update-ca-trust; }"
}

func caInstallPS(path string) string {
	return "certutil -addstore -f Root " + psSingleQuote(path)
}

func (e *Engine) installCAUnprivileged(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = newCmd("security", "add-trusted-cert", "-r", "trustRoot",
			"-k", os.Getenv("HOME")+"/Library/Keychains/login.keychain-db", path)
	case "windows":
		cmd = newCmd("certutil", "-addstore", "-f", "Root", path)
	case "linux":
		cmd = newCmd("sh", "-c", caInstallSh(path))
	default:
		return fmt.Errorf("unsupported OS %s", runtime.GOOS)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// cleanup removes the staged cert; it must outlive the elevated call, so caStep
// does not remove it itself.
func (e *Engine) caStep(host string) (privilegedStep, func(), bool) {
	noop := func() {}
	if e.hostTrustsCARE(host) {
		return privilegedStep{}, noop, false
	}
	pem := e.caddyRootPEM()
	if pem == "" {
		return privilegedStep{}, noop, false // caddy not ready or no root yet
	}
	f, err := os.CreateTemp("", "care-root-*.crt")
	if err != nil {
		return privilegedStep{}, noop, false
	}
	path := f.Name()
	if _, err := f.WriteString(pem); err != nil {
		f.Close()
		os.Remove(path)
		return privilegedStep{}, noop, false
	}
	f.Close()
	cleanup := func() { os.Remove(path) }

	if err := e.installCAUnprivileged(path); err == nil {
		e.logln("This machine now trusts https://" + host + "/.")
		return privilegedStep{}, cleanup, false
	}
	return privilegedStep{
		what: "trust CARE's security certificate, so the browser shows no warning",
		sh:   caInstallSh(path),
		ps:   caInstallPS(path),
	}, cleanup, true
}

// No InsecureSkipVerify on purpose: we want the real system verdict.
func (e *Engine) hostTrustsCARE(host string) bool {
	c := &http.Client{Timeout: 4 * time.Second}
	resp, err := c.Get("https://" + host + "/ping/")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

const caddyRootPath = "/data/caddy/pki/authorities/local/root.crt"

// Must run BEFORE `compose down -v` destroys caddy-data. Two ways in because
// uninstall gets one attempt: exec, then cp if the container won't take an exec.
func (e *Engine) caddyRootPEM() string {
	if out, err := e.capture("docker", "compose", "exec", "-T", "caddy",
		"cat", caddyRootPath); err == nil && strings.Contains(out, "BEGIN CERTIFICATE") {
		return out
	}
	f, err := os.CreateTemp("", "care-root-*.crt")
	if err != nil {
		return ""
	}
	tmp := f.Name()
	f.Close()
	defer os.Remove(tmp)
	if _, err := e.capture("docker", "compose", "cp", "caddy:"+caddyRootPath, tmp); err != nil {
		return ""
	}
	b, err := os.ReadFile(tmp)
	if err != nil || !strings.Contains(string(b), "BEGIN CERTIFICATE") {
		return ""
	}
	return string(b)
}

// Set by our Caddyfile's pki block, so a cert carrying it is ours by construction.
const caCommonName = "CARE Desktop Local CA"

var linuxCAAnchors = []string{
	"/usr/local/share/ca-certificates/care-root.crt",
	"/etc/pki/ca-trust/source/anchors/care-root.crt",
}

// Sweeps by CN as well as fingerprint: setup mints a new root each install, so
// matching only the current one leaves every earlier root trusted forever.
func (e *Engine) untrustLocalCA(pem string) {
	fp := certSHA1Hex(pem)
	removed, err := e.removeTrustedRoots(fp)
	switch {
	case err != nil:
		e.logln("Could not remove CARE's certificate from this machine's trust store (" +
			err.Error() + "). Remove \"" + caCommonName + "\" by hand if you want it gone.")
	case removed:
		e.logln("Removed CARE's certificate from this machine's trust store.")
	case fp == "":
		e.logln("Note: couldn't read CARE's certificate before teardown, and found none to remove. " +
			"If a browser still trusts \"" + caCommonName + "\", remove it by hand.")
	}
}

func (e *Engine) removeTrustedRoots(fp string) (bool, error) {
	switch runtime.GOOS {
	case "darwin":
		return e.removeTrustedRootsDarwin(fp)
	case "windows":
		return e.removeTrustedRootsWindows(fp)
	case "linux":
		return e.removeTrustedRootsLinux()
	}
	return false, nil
}

// System keychain is read first (free) so an empty store raises no admin prompt.
func (e *Engine) removeTrustedRootsDarwin(fp string) (bool, error) {
	login := os.Getenv("HOME") + "/Library/Keychains/login.keychain-db"
	removed := false
	for _, h := range darwinCARoots(login, fp) {
		if newCmd("security", "delete-certificate", "-Z", h, login).Run() == nil {
			removed = true
		}
	}

	const sys = "/Library/Keychains/System.keychain"
	hashes := darwinCARoots(sys, fp)
	if len(hashes) == 0 {
		return removed, nil
	}
	if e.Confirm == nil || !e.Confirm("Remove CARE's certificate?",
		"Remove CARE's security certificate from this computer's System keychain?\n\nThis needs administrator approval.") {
		return removed, nil
	}
	cmds := make([]string, 0, len(hashes))
	for _, h := range hashes {
		cmds = append(cmds, "security delete-certificate -Z "+h+" "+sys)
	}
	if err := e.runPrivileged(strings.Join(cmds, "; "), true); err != nil {
		return removed, err
	}
	return true, nil
}

func darwinCARoots(keychain, fp string) []string {
	var raw string
	if b, err := newCmd("security", "find-certificate", "-a", "-Z",
		"-c", caCommonName, keychain).Output(); err == nil {
		raw = string(b)
	}
	hashes := parseSHA1Hashes(raw)
	if fp != "" && certInKeychain(keychain, fp) {
		hashes = appendUnique(hashes, fp)
	}
	return hashes
}

func parseSHA1Hashes(raw string) []string {
	var out []string
	for _, ln := range strings.Split(raw, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(ln), "SHA-1 hash:"); ok {
			out = appendUnique(out, rest)
		}
	}
	return out
}

func appendUnique(list []string, h string) []string {
	h = strings.ToUpper(strings.TrimSpace(h))
	if h == "" {
		return list
	}
	for _, existing := range list {
		if existing == h {
			return list
		}
	}
	return append(list, h)
}

// One UAC prompt; presence checked first (readable without admin) so an uninstall
// with nothing of ours never prompts.
func (e *Engine) removeTrustedRootsWindows(fp string) (bool, error) {
	if !windowsRootPresent(fp) {
		return false, nil
	}
	cmds := []string{}
	if fp != "" {
		cmds = append(cmds, "certutil -delstore Root "+psSingleQuote(fp))
	}
	cmds = append(cmds, "certutil -delstore Root "+psSingleQuote(caCommonName))
	inner := strings.Join(cmds, "; ")
	ps := "Start-Process powershell -Verb RunAs -Wait -ArgumentList '-NoProfile','-Command'," +
		psSingleQuote(inner)
	if err := newCmd("powershell", "-NoProfile", "-Command", ps).Run(); err != nil {
		return false, err
	}
	return true, nil
}

func windowsRootPresent(fp string) bool {
	out, err := newCmd("certutil", "-store", "Root").Output()
	if err != nil {
		return fp != "" // can't tell - only bother if we actually captured a root
	}
	s := strings.ToUpper(string(out))
	if strings.Contains(s, strings.ToUpper(caCommonName)) {
		return true
	}
	// certutil prints fingerprints byte-spaced; compare without the spaces.
	return fp != "" && strings.Contains(strings.ReplaceAll(s, " ", ""), strings.ToUpper(fp))
}

// File-based, so nothing accumulates and there is nothing to sweep by name.
func (e *Engine) removeTrustedRootsLinux() (bool, error) {
	present := false
	for _, p := range linuxCAAnchors {
		if fileExists(p) {
			present = true
		}
	}
	if !present {
		return false, nil
	}
	sh := "rm -f " + strings.Join(linuxCAAnchors, " ") + "; " +
		"update-ca-certificates 2>/dev/null; update-ca-trust 2>/dev/null; true"
	if newCmd("sh", "-c", sh).Run() == nil && !fileExists(linuxCAAnchors[0]) {
		return true, nil
	}
	if e.Confirm == nil || !e.Confirm("Remove CARE's certificate?",
		"Remove CARE's security certificate from this computer?\n\nThis needs administrator approval.") {
		return false, nil
	}
	if err := e.runPrivileged(sh, true); err != nil {
		return false, err
	}
	return true, nil
}

// The fingerprint macOS `security` and Windows `certutil` both match on.
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

func certInKeychain(keychain, fp string) bool {
	out, err := newCmd("security", "find-certificate", "-a", "-Z", keychain).Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToUpper(string(out)), strings.ToUpper(fp))
}
