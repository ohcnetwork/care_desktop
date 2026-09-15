package trust

import (
	"os"
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
			dbs := []string{filepath.Join(home, ".pki", "nssdb"), filepath.Join(home, ".mozilla", "firefox", "clinic name.default-release")}
			for _, p := range append([]string{bin, anchorDir}, dbs...) {
				if err := os.MkdirAll(p, 0o755); err != nil {
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
			out, err := proc.Command("sh", "-c", script).CombinedOutput()
			if tc.fail {
				if err == nil || strings.Contains(string(out), "Done -") || !strings.Contains(string(out), "browser certificate import failed") {
					t.Fatalf("NSS failure was not reported: %v, %s", err, out)
				}
				return
			}
			if err != nil || !strings.Contains(string(out), "Done -") {
				t.Fatalf("installation failed: %v, %s", err, out)
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
