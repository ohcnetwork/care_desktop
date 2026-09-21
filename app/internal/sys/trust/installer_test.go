package trust

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/elevate"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

func TestUnixInstallerUsesReadableAnchorAndReportsNSSFailures(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell fixture")
	}
	for _, tc := range []struct {
		name string
		fail bool
	}{
		{"Debian", false},
		{"Fedora", false},
		{"failed NSS import", true},
		{"root login", false},
		{"missing certutil", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "bin")
			home := filepath.Join(dir, "user home")
			debian := filepath.Join(dir, "debian anchors")
			fedora := filepath.Join(dir, "fedora anchors")
			anchorDir := debian
			if tc.name == "Fedora" {
				anchorDir = fedora
			}
			dbs := []string{
				filepath.Join(home, ".pki", "nssdb"),
				filepath.Join(home, ".mozilla", "firefox", "clinic name.default-release"),
				filepath.Join(home, ".mozilla", "firefox", "custom-profile"),
			}
			for _, p := range append([]string{bin, anchorDir}, dbs...) {
				if err := os.MkdirAll(p, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for _, db := range dbs {
				if err := os.WriteFile(filepath.Join(db, "cert9.db"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for name, body := range map[string]string{
				"id":                     "printf '0\\n'\n",
				"uname":                  "printf 'Linux\\n'\n",
				"update-ca-certificates": "exit 0\n",
				"update-ca-trust":        "exit 0\n",
				"security":               "exit 88\n",
				"sudo":                   "[ \"$1\" = '-H' ] && [ \"$2\" = '-u' ] && [ \"$3\" = 'fixture' ] || exit 88\nshift 3\nexec \"$@\"\n",
				"certutil": `while [ "$#" -gt 0 ]; do
  case "$1" in
    -i) shift; pem=$1 ;;
    -d) shift; db=$1 ;;
  esac
  shift
done
[ "$pem" = "$EXPECTED_ANCHOR" ] || exit 89
case "$(LC_ALL=C ls -ld "$pem")" in
  ???????r??*) ;;
  *) echo "certificate is not publicly readable" >&2; exit 90 ;;
esac
printf '%s\n' "$db" >> "$IMPORT_TRACE"
[ "$FAIL_IMPORT" != yes ]
`,
			} {
				if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("HOME", home)
			t.Setenv("SUDO_USER", "fixture")
			if tc.name == "root login" {
				t.Setenv("SUDO_USER", "")
			}
			t.Setenv("TMPDIR", dir)
			t.Setenv("EXPECTED_ANCHOR", filepath.Join(anchorDir, "care-root.crt"))
			trace := filepath.Join(dir, "imports")
			t.Setenv("IMPORT_TRACE", trace)
			fail := "no"
			if tc.fail {
				fail = "yes"
			}
			t.Setenv("FAIL_IMPORT", fail)
			script := UnixInstaller("public certificate fixture", "fixture", "care.local")
			script = strings.ReplaceAll(script, "/usr/local/share/ca-certificates", elevate.ShQuote(debian))
			script = strings.ReplaceAll(script, "/etc/pki/ca-trust/source/anchors", elevate.ShQuote(fedora))
			if tc.name == "missing certutil" {
				script = strings.ReplaceAll(script, "command -v certutil", "command -v care-test-missing-certutil")
			}
			out, err := proc.Command("sh", "-c", script).CombinedOutput()
			if tc.fail {
				if err == nil || strings.Contains(string(out), "Certificate installed") || !strings.Contains(string(out), "browser certificate import failed") {
					t.Fatalf("NSS failure was not reported: %v, %s", err, out)
				}
				return
			}
			if err != nil || !strings.Contains(string(out), "Certificate installed") {
				t.Fatalf("installation failed: %v, %s", err, out)
			}
			if tc.name == "root login" || tc.name == "missing certutil" {
				if !strings.Contains(string(out), "Browser certificates were not updated:") {
					t.Fatalf("incomplete browser setup was not reported: %s", out)
				}
				if _, err := os.Stat(trace); !os.IsNotExist(err) {
					t.Fatalf("unexpected browser imports: %v", err)
				}
				return
			}
			imports, err := os.ReadFile(trace)
			if err != nil {
				t.Fatal(err)
			}
			for _, db := range dbs {
				if !strings.Contains(string(imports), "sql:"+db+"\n") {
					t.Errorf("browser store %q was not imported: %s", db, imports)
				}
			}
		})
	}
}

func TestUnixInstallerMacImportFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell fixture")
	}
	dir := t.TempDir()
	for name, body := range map[string]string{
		"id":       "echo 0",
		"uname":    "echo Darwin",
		"security": "echo 'certificate import denied' >&2; exit 1",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TMPDIR", dir)
	out, err := proc.Command("sh", "-c", UnixInstaller("fixture", "fixture", "care.local")).CombinedOutput()
	if err == nil || !strings.Contains(string(out), "certificate import denied") || strings.Contains(string(out), "Certificate installed") {
		t.Fatalf("Mac import failure was not preserved: %v, %s", err, out)
	}
}

func TestWindowsInstallerReportsFailures(t *testing.T) {
	shell, err := exec.LookPath("powershell")
	if err != nil {
		shell, err = exec.LookPath("pwsh")
	}
	if err != nil {
		t.Skip("PowerShell is needed to execute installer fixtures")
	}
	for _, tc := range []struct {
		name, admin, start, importCert, want string
		fail                                 bool
	}{
		{"installed", "$true", "throw 'unexpected elevation'", "", "Certificate installed", false},
		{"import denied", "$true", "throw 'unexpected elevation'", "throw 'fixture import denied'", "fixture import denied", true},
		{"elevation cancelled", "$false", "throw 'fixture approval cancelled'", "throw 'unexpected import'", "fixture approval cancelled", true},
		{"elevated child failed", "$false", "return [pscustomobject]@{ ExitCode = 23 }", "throw 'unexpected import'", "exit code 23", true},
		{"elevated child succeeded", "$false", "return [pscustomobject]@{ ExitCode = 0 }", "throw 'unexpected import'", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			script := WindowsInstaller("certificate fixture", "fixture", "care.local")
			start := strings.Index(script, "  $identity =")
			end := strings.Index(script, "  if (-not $isAdmin)")
			if start < 0 || end < start {
				t.Fatal("could not replace platform identity check")
			}
			script = script[:start] + "  $isAdmin = " + tc.admin + "\n" + script[end:]
			script = `function Start-Process {
  param($FilePath, $Verb, [switch]$Wait, [switch]$PassThru, $ArgumentList)
  if (-not $Wait -or -not $PassThru -or $Verb -ne 'RunAs' -or
      $ArgumentList[-1] -ne ('"' + $PSCommandPath + '"')) {
    throw 'invalid elevation arguments'
  }
  ` + tc.start + "\n}\n" +
				`function Import-Certificate {
  param($FilePath, $CertStoreLocation)
  if ((Get-Content -LiteralPath $FilePath -Raw).Trim() -ne 'certificate fixture' -or
      $CertStoreLocation -ne 'Cert:\LocalMachine\Root') {
    throw 'invalid certificate import'
  }
  ` + tc.importCert + "\n}\n" +
				"function Read-Host { Write-Host 'fixture paused' }\n" + script
			path := filepath.Join(t.TempDir(), "installer with spaces.ps1")
			if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
				t.Fatal(err)
			}
			out, err := proc.Command(shell, "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", path).CombinedOutput()
			if (err != nil) != tc.fail || !strings.Contains(string(out), tc.want) {
				t.Fatalf("unexpected installer result: %v, %s", err, out)
			}
			if tc.fail && (!strings.Contains(string(out), "fixture paused") ||
				!strings.Contains(string(out), "http://care.local/setup#windows") ||
				strings.Contains(string(out), "Certificate installed")) {
				t.Fatalf("failure did not stay visible without a success message: %s", out)
			}
		})
	}
}
