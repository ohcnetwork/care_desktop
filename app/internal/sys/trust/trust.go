package trust

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

	"github.com/ohcnetwork/care_desktop/app/internal/sys/elevate"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

func installSh(path string) string {
	if runtime.GOOS == "darwin" {
		return "security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain " +
			elevate.ShQuote(path)
	}
	q := elevate.ShQuote(path)
	return "cp " + q + " /usr/local/share/ca-certificates/care-root.crt && update-ca-certificates " +
		"|| { cp " + q + " /etc/pki/ca-trust/source/anchors/care-root.crt && update-ca-trust; }"
}

func installPS(path string) string {
	return "certutil -addstore -f Root " + elevate.PSQuote(path)
}

func installUnprivileged(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = proc.Command("security", "add-trusted-cert", "-r", "trustRoot",
			"-k", os.Getenv("HOME")+"/Library/Keychains/login.keychain-db", path)
	case "windows":
		cmd = proc.Command("certutil", "-addstore", "-f", "Root", path)
	case "linux":
		cmd = proc.Command("sh", "-c", installSh(path))
	default:
		return fmt.Errorf("unsupported OS %s", runtime.GOOS)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func Step(log func(string), host, rootPEM string) (elevate.Step, func(), bool) {
	noop := func() {}
	if HostTrusts(host) {
		return elevate.Step{}, noop, false
	}
	if rootPEM == "" {
		return elevate.Step{}, noop, false // caddy not ready or no root yet
	}
	f, err := os.CreateTemp("", "care-root-*.crt")
	if err != nil {
		return elevate.Step{}, noop, false
	}
	path := f.Name()
	if _, err := f.WriteString(rootPEM); err != nil {
		f.Close()
		os.Remove(path)
		return elevate.Step{}, noop, false
	}
	f.Close()
	cleanup := func() { os.Remove(path) }

	if err := installUnprivileged(path); err == nil {
		logln(log, "This machine now trusts https://"+host+"/.")
		return elevate.Step{}, cleanup, false
	}
	return elevate.Step{
		What: "trust CARE's security certificate, so the browser shows no warning",
		Sh:   installSh(path),
		PS:   installPS(path),
	}, cleanup, true
}

func HostTrusts(host string) bool {
	c := &http.Client{Timeout: 4 * time.Second}
	resp, err := c.Get("https://" + host + "/ping/")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// Set by our Caddyfile's pki block, so a cert carrying it is ours by construction.
const CommonName = "CARE Desktop Local CA"

var linuxCAAnchors = []string{
	"/usr/local/share/ca-certificates/care-root.crt",
	"/etc/pki/ca-trust/source/anchors/care-root.crt",
}

func Untrust(log func(string), confirm func(string, string) bool, rootPEM string) string {
	fp := SHA1Hex(rootPEM)
	removed, err := removeTrustedRoots(confirm, fp)
	if Present() {
		detail := ""
		if err != nil {
			detail = " (" + err.Error() + ")"
		}
		logln(log, "Could not remove CARE's certificate from this machine's trust store"+detail+
			". Remove \""+CommonName+"\" by hand if you want it gone.")
		return "The certificate \"" + CommonName + "\" is still trusted by this computer" + detail +
			". " + manualRemoval()
	}
	switch {
	case removed:
		logln(log, "Removed CARE's certificate from this machine's trust store.")
	case fp == "":
		logln(log, "Note: couldn't read CARE's certificate before teardown, and found none to remove. "+
			"If a browser still trusts \""+CommonName+"\", remove it by hand.")
	}
	return ""
}

func manualRemoval() string {
	switch runtime.GOOS {
	case "darwin":
		return "Remove it in Keychain Access, or run: security delete-certificate -c " +
			elevate.ShQuote(CommonName)
	case "windows":
		return "Remove it in certmgr.msc, or run as administrator: certutil -delstore Root " +
			elevate.PSQuote(CommonName)
	}
	return "Delete " + strings.Join(linuxCAAnchors, " and ") + ", then run update-ca-certificates."
}

func removeTrustedRoots(confirm func(string, string) bool, fp string) (bool, error) {
	switch runtime.GOOS {
	case "darwin":
		return removeTrustedRootsDarwin(confirm, fp)
	case "windows":
		return removeTrustedRootsWindows(fp)
	case "linux":
		return removeTrustedRootsLinux(confirm)
	}
	return false, nil
}

func removeTrustedRootsDarwin(confirm func(string, string) bool, fp string) (bool, error) {
	login := os.Getenv("HOME") + "/Library/Keychains/login.keychain-db"
	removed := false
	for _, h := range darwinCARoots(login, fp) {
		if proc.Command("security", "delete-certificate", "-Z", h, login).Run() == nil {
			removed = true
		}
	}

	const sys = "/Library/Keychains/System.keychain"
	hashes := darwinCARoots(sys, fp)
	if len(hashes) == 0 {
		return removed, nil
	}
	if confirm == nil || !confirm("Remove CARE's certificate?",
		"Remove CARE's security certificate from this computer's System keychain?\n\nThis needs administrator approval.") {
		return removed, nil
	}
	cmds := make([]string, 0, len(hashes))
	for _, h := range hashes {
		cmds = append(cmds, "security delete-certificate -Z "+h+" "+sys)
	}
	if err := elevate.Run(strings.Join(cmds, "; "), true); err != nil {
		return removed, err
	}
	return true, nil
}

func darwinCARoots(keychain, fp string) []string {
	var raw string
	if b, err := proc.Command("security", "find-certificate", "-a", "-Z",
		"-c", CommonName, keychain).Output(); err == nil {
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

func removeTrustedRootsWindows(fp string) (bool, error) {
	if !windowsRootPresent(fp) {
		return false, nil
	}
	cmds := []string{}
	if fp != "" {
		cmds = append(cmds, "certutil -delstore Root "+elevate.PSQuote(fp))
	}
	cmds = append(cmds, "certutil -delstore Root "+elevate.PSQuote(CommonName))
	inner := strings.Join(cmds, "; ")
	ps := "Start-Process powershell -Verb RunAs -Wait -ArgumentList '-NoProfile','-Command'," +
		elevate.PSQuote(inner)
	if err := proc.Command("powershell", "-NoProfile", "-Command", ps).Run(); err != nil {
		return false, err
	}
	return true, nil
}

func windowsRootPresent(fp string) bool {
	out, err := proc.Command("certutil", "-store", "Root").Output()
	if err != nil {
		return fp != "" // can't tell - only bother if we actually captured a root
	}
	s := strings.ToUpper(string(out))
	if strings.Contains(s, strings.ToUpper(CommonName)) {
		return true
	}
	return fp != "" && strings.Contains(strings.ReplaceAll(s, " ", ""), strings.ToUpper(fp))
}

func removeTrustedRootsLinux(confirm func(string, string) bool) (bool, error) {
	present := false
	for _, p := range linuxCAAnchors {
		if proc.FileExists(p) {
			present = true
		}
	}
	if !present {
		return false, nil
	}
	sh := "rm -f " + strings.Join(linuxCAAnchors, " ") + "; " +
		"update-ca-certificates 2>/dev/null; update-ca-trust 2>/dev/null; true"
	if proc.Command("sh", "-c", sh).Run() == nil && !proc.FileExists(linuxCAAnchors[0]) {
		return true, nil
	}
	if confirm == nil || !confirm("Remove CARE's certificate?",
		"Remove CARE's security certificate from this computer?\n\nThis needs administrator approval.") {
		return false, nil
	}
	if err := elevate.Run(sh, true); err != nil {
		return false, err
	}
	return true, nil
}

func SHA1Hex(pemData string) string {
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
	out, err := proc.Command("security", "find-certificate", "-a", "-Z", keychain).Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToUpper(string(out)), strings.ToUpper(fp))
}

func logln(log func(string), s string) {
	if log != nil {
		log(s)
	}
}

func Present() bool {
	switch runtime.GOOS {
	case "darwin":
		login := os.Getenv("HOME") + "/Library/Keychains/login.keychain-db"
		const sys = "/Library/Keychains/System.keychain"
		return len(darwinCARoots(login, "")) > 0 || len(darwinCARoots(sys, "")) > 0
	case "windows":
		return windowsRootPresent("")
	case "linux":
		for _, p := range linuxCAAnchors {
			if proc.FileExists(p) {
				return true
			}
		}
	}
	return false
}
