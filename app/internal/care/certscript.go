package care

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
)

// Renders the /setup page's one-click trust installers with the root PEM embedded.
// Embedded, not fetched: a device that doesn't trust us yet can't fetch over https,
// and we don't want an installer that shrugs off TLS errors.
// See docs/architecture.md#certificate-trust--the-setup-bootstrap.
func (e *Engine) writeCertInstallers() {
	root := e.caddyRootPEM()
	if root == "" {
		return
	}
	dir := filepath.Join(e.Kit, "setup")
	if _, err := os.Stat(dir); err != nil {
		return
	}
	fp := certSHA256Colons(root)
	host := e.host()

	files := map[string]string{
		"install-cert.sh":  unixInstaller(root, fp, host),
		"install-cert.ps1": windowsInstaller(root, fp, host),
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			e.logln("note: couldn't write " + name + " (" + err.Error() + ")")
		}
	}
}

// certSHA256Colons formats the fingerprint the way OS cert viewers show it, so a
// user can compare what the script prints against the /setup page.
func certSHA256Colons(pemData string) string {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil || block.Type != "CERTIFICATE" {
		return ""
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return ""
	}
	sum := sha256.Sum256(block.Bytes)
	h := strings.ToUpper(hex.EncodeToString(sum[:]))
	var parts []string
	for i := 0; i < len(h); i += 2 {
		parts = append(parts, h[i:i+2])
	}
	return strings.Join(parts, ":")
}

func unixInstaller(root, fp, host string) string {
	return `#!/bin/sh
# Trusts this clinic's certificate on macOS or Linux, so https://` + host + `
# opens without warnings. Run it with:   sh install-cert.sh
#
# Certificate SHA-256: ` + fp + `
set -eu

if [ "$(id -u)" -ne 0 ]; then
  echo "Adding a certificate needs administrator rights - you'll be asked for your password."
  exec sudo sh "$0" "$@"
fi

PEM=$(mktemp)
trap 'rm -f "$PEM"' EXIT INT TERM
cat > "$PEM" <<'CARE_ROOT_PEM'
` + strings.TrimSpace(root) + `
CARE_ROOT_PEM

case "$(uname -s)" in
Darwin)
  security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain "$PEM"
  ;;
Linux)
  # chmod: mktemp makes the copy 0600, but a CA anchor must be world-readable.
  if [ -d /usr/local/share/ca-certificates ]; then
    cp "$PEM" /usr/local/share/ca-certificates/care-root.crt
    chmod 644 /usr/local/share/ca-certificates/care-root.crt
    update-ca-certificates >/dev/null
  elif [ -d /etc/pki/ca-trust/source/anchors ]; then
    cp "$PEM" /etc/pki/ca-trust/source/anchors/care-root.crt
    chmod 644 /etc/pki/ca-trust/source/anchors/care-root.crt
    update-ca-trust
  else
    echo "Couldn't find this system's certificate directory. Install it by hand:" >&2
    echo "  https://` + host + `/setup" >&2
    exit 1
  fi
  # Firefox and Chrome keep their own store, so the system one isn't enough.
  if [ -n "${SUDO_USER:-}" ] && command -v certutil >/dev/null 2>&1; then
    for db in $(sudo -u "$SUDO_USER" sh -c 'ls -d ~/.pki/nssdb ~/.mozilla/firefox/*.default* 2>/dev/null' || true); do
      sudo -u "$SUDO_USER" certutil -d "sql:$db" -A -t "C,," -n "CARE Desktop Local CA" -i "$PEM" 2>/dev/null || true
    done
  fi
  ;;
*)
  echo "Unsupported system: $(uname -s)" >&2
  exit 1
  ;;
esac

echo
echo "Done - this computer now trusts the clinic."
echo "Reopen your browser and visit https://` + host + `/"
`
}

func windowsInstaller(root, fp, host string) string {
	return `# Trusts this clinic's certificate on Windows, so https://` + host + `
# opens without warnings. Right-click this file and choose "Run with PowerShell".
#
# Certificate SHA-256: ` + fp + `
$ErrorActionPreference = 'Stop'

$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$isAdmin = (New-Object Security.Principal.WindowsPrincipal $identity).IsInRole(
  [Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
  Write-Host "Adding a certificate needs administrator rights - approve the prompt."
  # Quoted by concatenation, not a backtick escape: this file is a Go raw string.
  $quoted = '"' + $PSCommandPath + '"'
  Start-Process powershell -Verb RunAs -Wait -ArgumentList @(
    '-NoProfile','-ExecutionPolicy','Bypass','-File',$quoted)
  exit
}

$pem = @'
` + strings.TrimSpace(root) + `
'@

$tmp = Join-Path $env:TEMP 'care-root.crt'
Set-Content -LiteralPath $tmp -Value $pem -Encoding ASCII
try {
  Import-Certificate -FilePath $tmp -CertStoreLocation Cert:\LocalMachine\Root | Out-Null
} finally {
  Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue
}

Write-Host ""
Write-Host "Done - this computer now trusts the clinic."
Write-Host "Reopen your browser and visit https://` + host + `/"
Write-Host ""
Read-Host "Press Enter to close"
`
}
