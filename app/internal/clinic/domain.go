package clinic

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/mdns"
)

// Files carrying the clinic address. Rewritten in place so two clinics can share a
// LAN. See docs/configuration.md#clinic-address.
var domainFiles = []string{"Caddyfile", "backend.env", "frontend.env", "setup/index.html"}

// Matches a bare "<label>.local" host. "localhost" and path segments like
// pki/authorities/local have no dot before "local", so they never match.
var hostRe = regexp.MustCompile(`\b[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?\.local\b`)

// ApplyDomain points the install dir at this clinic's address. Runs before
// BuildFrontend during setup - Vite bakes the API URL in at build time, so a later
// change needs a rebuild - and again on every launch, after the install files are
// refreshed from the binary, because the shipped templates carry example.local.
//
// Idempotent: it replaces whatever host is already there rather than a fixed
// string, and skips any file it would not change.
func (e *Clinic) ApplyDomain() error {
	if err := mdns.ValidateLabel(e.mdnsName()); err != nil {
		return err
	}
	host := e.host()
	for _, rel := range domainFiles {
		path := filepath.Join(e.InstallDir, filepath.FromSlash(rel))
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

// configuredHost is the address the install dir currently serves, "" if unknown.
func (e *Clinic) configuredHost() string {
	b, err := os.ReadFile(filepath.Join(e.InstallDir, "Caddyfile"))
	if err != nil {
		return ""
	}
	return hostRe.FindString(string(b))
}

// warnDomainDrift catches a start where the name changed but setup never re-ran:
// the proxy still answers for the old address while the hosts entry points at the
// new one, which serves nothing. Applying it here wouldn't help, since the
// frontend bakes its API URL at build time.
func (e *Clinic) warnDomainDrift() {
	if cur := e.configuredHost(); cur != "" && cur != e.host() {
		e.logln("warning: this install still serves https://" + cur + "/ but the address is set to " +
			e.host() + ". Run setup again so the whole stack matches.")
	}
}

// host is the address devices use: lowercase, with exactly one ".local".
func (e *Clinic) host() string {
	return mdns.Label(e.mdnsName()) + ".local"
}

// Host is the clinic address, e.g. "care.local".
func (e *Clinic) Host() string { return e.host() }
