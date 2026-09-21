package trust

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"strings"
)

func SHA256Colons(pemData string) string {
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

func UnixInstaller(root, fp, host string) string {
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
  if [ -d /usr/local/share/ca-certificates ]; then
    ANCHOR=/usr/local/share/ca-certificates/care-root.crt
    cp "$PEM" "$ANCHOR"
    chmod 644 "$ANCHOR"
    update-ca-certificates >/dev/null
  elif [ -d /etc/pki/ca-trust/source/anchors ]; then
    ANCHOR=/etc/pki/ca-trust/source/anchors/care-root.crt
    cp "$PEM" "$ANCHOR"
    chmod 644 "$ANCHOR"
    update-ca-trust
  else
    echo "Couldn't find this system's certificate directory. Install it by hand:" >&2
    echo "  https://` + host + `/setup" >&2
    exit 1
  fi
  # Some browsers use their own store in addition to system trust.
  if [ -z "${SUDO_USER:-}" ]; then
    echo "Browser certificates were not updated: run this installer from your normal user account with sudo, not from a root login." >&2
  elif ! command -v certutil >/dev/null 2>&1; then
    echo "Browser certificates were not updated: certutil is missing. Ask your administrator to install libnss3-tools (Debian/Ubuntu) or nss-tools (Fedora/RHEL), then run this installer again." >&2
  else
    sudo -H -u "$SUDO_USER" sh -c '
      for file in "$HOME/.pki/nssdb/cert9.db" "$HOME"/.mozilla/firefox/*/cert9.db; do
        [ -f "$file" ] || continue
        db=${file%/cert9.db}
        if ! certutil -d "sql:$db" -A -t "C,," -n "CARE Desktop Local CA" -i "$1"; then
          echo "System trust was updated, but browser certificate import failed for $db. Close the browser and run this installer again." >&2
          exit 1
        fi
      done
    ' sh "$ANCHOR"
  fi
  ;;
*)
  echo "Unsupported system: $(uname -s)" >&2
  exit 1
  ;;
esac

echo
echo "Certificate installed in this computer's system trust store."
echo "Reopen your browser and visit https://` + host + `/ to check the connection."
echo "If a security warning remains, stop and ask your clinic administrator. Do not bypass it."
`
}

func WindowsInstaller(root, fp, host string) string {
	return `# Trusts this clinic's certificate on Windows, so https://` + host + `
# opens without warnings. Right-click this file and choose "Run with PowerShell".
#
# Certificate SHA-256: ` + fp + `
$ErrorActionPreference = 'Stop'
$exitCode = 0

try {
  $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
  $isAdmin = (New-Object Security.Principal.WindowsPrincipal $identity).IsInRole(
    [Security.Principal.WindowsBuiltInRole]::Administrator)
  if (-not $isAdmin) {
    Write-Host "Adding a certificate needs administrator rights - approve the prompt."
    # Quoted by concatenation, not a backtick escape: this file is a Go raw string.
    $quoted = '"' + $PSCommandPath + '"'
    $process = Start-Process powershell -Verb RunAs -Wait -PassThru -ArgumentList @(
      '-NoProfile','-ExecutionPolicy','Bypass','-File',$quoted)
    if ($process.ExitCode -ne 0) {
      throw "Administrator setup did not finish (exit code $($process.ExitCode))."
    }
    exit 0
  }

  $pem = @'
` + strings.TrimSpace(root) + `
'@

  $tmp = [IO.Path]::GetTempFileName()
  try {
    Set-Content -LiteralPath $tmp -Value $pem -Encoding ASCII
    Import-Certificate -FilePath $tmp -CertStoreLocation Cert:\LocalMachine\Root | Out-Null
  } finally {
    Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue
  }

  Write-Host ""
  Write-Host "Certificate installed in this computer's system trust store."
  Write-Host "Reopen your browser and visit https://` + host + `/ to check the connection."
  Write-Host "If a security warning remains, stop and ask your clinic administrator. Do not bypass it."
} catch {
  $exitCode = 1
  Write-Host ""
  Write-Host "CARE setup could not finish." -ForegroundColor Red
  Write-Host $_.Exception.Message -ForegroundColor Red
  Write-Host "Share this message with your clinic administrator, or use the manual instructions:"
  Write-Host "http://` + host + `/setup#windows"
}

Write-Host ""
Read-Host "Press Enter to close" | Out-Null
exit $exitCode
`
}
