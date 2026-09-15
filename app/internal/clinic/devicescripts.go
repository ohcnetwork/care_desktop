package clinic

import (
	"os"
	"path/filepath"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/trust"
)

func (e *Clinic) writeDeviceSetupScripts() {
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
