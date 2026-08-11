package care

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Files carrying the clinic address. Rewritten in place so two clinics can share a
// LAN. See docs/configuration.md#clinic-address.
var domainFiles = []string{"Caddyfile", "backend.env", "frontend.env", "setup/index.html"}

// Matches a bare "<label>.local" host. "localhost" and path segments like
// pki/authorities/local have no dot before "local", so they never match.
var hostRe = regexp.MustCompile(`\b[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?\.local\b`)

var labelRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)

// Exported for the installer, which checks as the user types.
func ValidateMDNSLabel(name string) error {
	label := strings.ToLower(mdnsLabel(name))
	switch {
	case label == "":
		return fmt.Errorf("enter a name, for example care")
	case len(label) > 63:
		return fmt.Errorf("name is too long (63 characters at most)")
	case !labelRe.MatchString(label):
		return fmt.Errorf("use lowercase letters, numbers and hyphens only, for example care-test")
	}
	return nil
}

// applyDomain points the kit at this clinic's address. Runs before buildFrontend:
// Vite bakes the API URL in at build time, so a later change needs a rebuild.
func (e *Engine) applyDomain() error {
	if err := ValidateMDNSLabel(e.mdnsName()); err != nil {
		return err
	}
	host := e.host()
	for _, rel := range domainFiles {
		path := filepath.Join(e.Kit, filepath.FromSlash(rel))
		b, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		// Replaces whatever host is there now, not a fixed "care.local", so
		// changing the name a second time still lands.
		out := hostRe.ReplaceAllString(string(b), host)
		if out == string(b) {
			continue
		}
		if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
			return err
		}
	}
	e.logln("Clinic address set to https://" + host + "/")
	return nil
}

// configuredHost is the address the kit currently serves, "" if unknown.
func (e *Engine) configuredHost() string {
	b, err := os.ReadFile(filepath.Join(e.Kit, "Caddyfile"))
	if err != nil {
		return ""
	}
	return hostRe.FindString(string(b))
}

// warnDomainDrift catches a start where the name changed but setup never re-ran:
// the proxy still answers for the old address while the hosts entry points at the
// new one, which serves nothing. Applying it here wouldn't help, since the
// frontend bakes its API URL at build time.
func (e *Engine) warnDomainDrift() {
	if cur := e.configuredHost(); cur != "" && cur != e.host() {
		e.logln("warning: this install still serves https://" + cur + "/ but the address is set to " +
			e.host() + ". Run setup again so the whole stack matches.")
	}
}
