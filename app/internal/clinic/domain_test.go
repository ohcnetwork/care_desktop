package clinic

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeDomainFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readDomainFile(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestApplyDomainPreservesUnmanagedSettings(t *testing.T) {
	dir := t.TempDir()
	caddy := "not header Referer *example.local*\nexample.local:443 {\n\ttls internal\n\timport bootstrap\n\timport site\n}\nreverse_proxy https://scanner.local\nreverse_proxy https://example.local.example.com\n"
	backend := "export BUCKET_EXTERNAL_ENDPOINT = \"https://example.local:443/files\"\r\n" +
		"CSRF_TRUSTED_ORIGINS='[\"https://example.local\", \"https://scanner.local\", \"https://example.local.other\"]'\r\n" +
		"PLUGIN_API=https://scanner.local/api\r\n" +
		"PLUGIN_CLINIC=https://example.local/api\r\n" +
		"ADDITIONAL_PLUGS='[\r\n  {\"name\":\"scanner\",\"configs\":{\"endpoint\":\"https://scanner.local\",\"clinic\":\"https://example.local\"}}\r\n]'\r\n" +
		"PLUGIN_TEMPLATE='first\r\nBUCKET_EXTERNAL_ENDPOINT=https://example.local\r\nlast'\r\n" +
		"PLUGIN_COLON: 'first\r\nBUCKET_EXTERNAL_ENDPOINT=https://example.local\r\nlast'\r\n"
	frontend := "REACT_CARE_API_URL=https://example.local\nPLUGIN_API=https://printer.local\nPLUGIN_CLINIC=https://example.local\n"
	setup := "<a href=\"https://example.local\">CARE</a><a href=\"https://printer.local\">Printer</a>"
	for name, content := range map[string]string{
		"Caddyfile":        caddy,
		"backend.env":      backend,
		"frontend.env":     frontend,
		"setup/index.html": setup,
	} {
		writeDomainFile(t, dir, name, content)
	}
	e := &Clinic{InstallDir: dir, MDNSName: "north"}
	if err := e.ApplyDomain(); err != nil {
		t.Fatal(err)
	}
	expectedBackend := strings.Replace(backend, "https://example.local:443/files", "https://north.local:443/files", 1)
	expectedBackend = strings.Replace(expectedBackend, `["https://example.local",`, `["https://north.local",`, 1)
	expected := map[string]string{
		"Caddyfile":        strings.Replace(strings.Replace(caddy, "*example.local*", "*north.local*", 1), "example.local:443 {", "north.local:443 {", 1),
		"backend.env":      expectedBackend,
		"frontend.env":     strings.Replace(frontend, "REACT_CARE_API_URL=https://example.local", "REACT_CARE_API_URL=https://north.local", 1),
		"setup/index.html": strings.Replace(setup, "https://example.local", "https://north.local", 1),
	}
	for name, want := range expected {
		if got := readDomainFile(t, dir, name); got != want {
			t.Errorf("%s:\ngot  %q\nwant %q", name, got, want)
		}
		info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
			t.Errorf("%s permissions changed to %v", name, info.Mode().Perm())
		}
	}
	if err := e.ApplyDomain(); err != nil {
		t.Fatal(err)
	}
	for name, want := range expected {
		if got := readDomainFile(t, dir, name); got != want {
			t.Errorf("%s was not idempotent", name)
		}
	}
	e.MDNSName = "south"
	if err := e.ApplyDomain(); err != nil {
		t.Fatal(err)
	}
	for name, previous := range expected {
		if got, want := readDomainFile(t, dir, name), strings.ReplaceAll(previous, "north.local", "south.local"); got != want {
			t.Errorf("%s retry:\ngot  %q\nwant %q", name, got, want)
		}
	}
}

func TestApplyDomainHandlesIncompleteSetup(t *testing.T) {
	dir := t.TempDir()
	e := &Clinic{InstallDir: dir, MDNSName: "first"}
	if err := e.ApplyDomain(); err != nil {
		t.Fatal(err)
	}
	writeDomainFile(t, dir, "Caddyfile", "example.local:443 {\n\ttls internal\n\timport bootstrap\n\timport site\n}\n")
	if err := e.ApplyDomain(); err != nil {
		t.Fatal(err)
	}
	e.MDNSName = "retry"
	if err := e.ApplyDomain(); err != nil {
		t.Fatal(err)
	}
	if got := readDomainFile(t, dir, "Caddyfile"); !strings.HasPrefix(got, "retry.local:443") {
		t.Fatalf("partial setup retained the old hostname: %s", got)
	}
	writeDomainFile(t, dir, "backend.env", "BUCKET_EXTERNAL_ENDPOINT=https://first.local\nCSRF_TRUSTED_ORIGINS=[\"https://first.local\", \"https://scanner.local\"]\n")
	if err := e.ApplyDomain(); err != nil {
		t.Fatal(err)
	}
	if got := readDomainFile(t, dir, "backend.env"); got != "BUCKET_EXTERNAL_ENDPOINT=https://retry.local\nCSRF_TRUSTED_ORIGINS=[\"https://retry.local\", \"https://scanner.local\"]\n" {
		t.Fatalf("preserved environment was not updated during retry: %s", got)
	}
	setup := "<p class=\"lede\">Install the certificate once to open <b>https://first.local</b> without warnings.</p>\n" +
		"<a href=\"https://scanner.local\">Scanner</a>\n"
	writeDomainFile(t, dir, "setup/index.html", setup)
	if err := e.ApplyDomain(); err != nil {
		t.Fatal(err)
	}
	if got, want := readDomainFile(t, dir, "setup/index.html"), strings.Replace(setup, "first.local", "retry.local", 1); got != want {
		t.Fatalf("partly updated setup page retained the old hostname: %s", got)
	}
}

func TestApplyDomainRejectsInvalidInputsBeforeWriting(t *testing.T) {
	for _, failure := range []string{"label", "read", "parse"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			e := &Clinic{InstallDir: dir, MDNSName: "north"}
			writeDomainFile(t, dir, "Caddyfile", "example.local:443 {}\n")
			switch failure {
			case "label":
				e.MDNSName = "not a label"
			case "read":
				if err := os.Mkdir(filepath.Join(dir, "backend.env"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "parse":
				writeDomainFile(t, dir, "backend.env", "BUCKET_EXTERNAL_ENDPOINT=\"unterminated\n")
			}
			if err := e.ApplyDomain(); err == nil {
				t.Fatal("invalid input was accepted")
			}
			if got := readDomainFile(t, dir, "Caddyfile"); got != "example.local:443 {}\n" {
				t.Fatal("domain files changed before input validation finished")
			}
		})
	}
}

func TestApplyDomainUpdatesDeviceGuide(t *testing.T) {
	page, err := os.ReadFile("../../../deployments/setup/index.html")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeDomainFile(t, dir, "setup/index.html", string(page))
	e := &Clinic{InstallDir: dir, MDNSName: "first"}
	for _, host := range []string{"first", "renamed"} {
		e.MDNSName = host
		if err := e.ApplyDomain(); err != nil {
			t.Fatal(err)
		}
		want := strings.ReplaceAll(string(page), "example.local", host+".local")
		if got := readDomainFile(t, dir, "setup/index.html"); got != want {
			t.Fatal("device guide did not update every clinic link")
		}
	}
}
