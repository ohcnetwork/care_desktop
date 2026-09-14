package clinic

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/mdns"
)

var domainFiles = []string{"Caddyfile", "backend.env", "frontend.env", "setup/index.html"}

var hostRe = regexp.MustCompile(`\b[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?\.local\b`)

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

func (e *Clinic) configuredHost() string {
	b, err := os.ReadFile(filepath.Join(e.InstallDir, "Caddyfile"))
	if err != nil {
		return ""
	}
	return hostRe.FindString(string(b))
}

func (e *Clinic) warnDomainDrift() {
	if cur := e.configuredHost(); cur != "" && cur != e.host() {
		e.logln("warning: this install still serves https://" + cur + "/ but the address is set to " +
			e.host() + ". Run setup again so the whole stack matches.")
	}
}

func (e *Clinic) host() string {
	return mdns.Label(e.mdnsName()) + ".local"
}

func (e *Clinic) Host() string { return e.host() }
