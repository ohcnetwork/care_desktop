package clinic

import (
	"os"
	"path/filepath"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/trust"
)

// Renders the /setup page's one-click trust installers with the root PEM embedded.
// Embedded, not fetched: a device that doesn't trust us yet can't fetch over https,
// and we don't want an installer that shrugs off TLS errors.
// See docs/architecture.md#certificate-trust--the-setup-bootstrap.
func (e *Clinic) writeCertInstallers() {
	root := e.caddyRootPEM()
	if root == "" {
		return
	}
	dir := filepath.Join(e.InstallDir, "setup")
	if _, err := os.Stat(dir); err != nil {
		return
	}
	fp := trust.SHA256Colons(root)
	host := e.host()

	files := map[string]string{
		"install-cert.sh":  trust.UnixInstaller(root, fp, host),
		"install-cert.ps1": trust.WindowsInstaller(root, fp, host),
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			e.logln("note: couldn't write " + name + " (" + err.Error() + ")")
		}
	}
}
