package care

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tlsEngine builds an Engine whose kit carries the given tls.env body.
func tlsEngine(t *testing.T, body string) *Engine {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tls.env"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return &Engine{Kit: dir}
}

const goodTLS = "CARE_PUBLIC_HOST=clinic.example.com\nCLOUDFLARE_API_TOKEN=tok\n"

func TestConfiguredOrigin(t *testing.T) {
	e := tlsEngine(t, goodTLS)
	if !e.Configured() {
		t.Fatal("host set but not reported as configured")
	}
	if got := e.PublicOrigin(); got != "https://clinic.example.com" {
		t.Fatalf("origin: %q", got)
	}
	if err := e.ValidateTLS(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}

	// Before the wizard runs there is no address at all, and nothing may pretend
	// otherwise - an empty origin must not become the string "https://".
	blank := tlsEngine(t, "CARE_PUBLIC_HOST=\nCLOUDFLARE_API_TOKEN=\n")
	if blank.Configured() || blank.PublicOrigin() != "" {
		t.Fatalf("unconfigured install: configured=%v origin=%q", blank.Configured(), blank.PublicOrigin())
	}
}

// There is no plain-HTTP fallback, so an install with no address must refuse to
// start rather than come up serving nothing.
func TestValidateTLSRejects(t *testing.T) {
	cases := []struct {
		name, body, wantIn string
	}{
		{"no host at all", "CARE_PUBLIC_HOST=\nCLOUDFLARE_API_TOKEN=tok\n", "no clinic web address set"},
		{"no token", "CARE_PUBLIC_HOST=clinic.example.com\n", "no Cloudflare API token"},
		{"dot-local", "CARE_PUBLIC_HOST=care.local\nCLOUDFLARE_API_TOKEN=tok\n", "reserved suffix"},
		{"a URL", "CARE_PUBLIC_HOST=https://clinic.example.com\nCLOUDFLARE_API_TOKEN=tok\n", "bare hostname"},
		{"no dot", "CARE_PUBLIC_HOST=clinic\nCLOUDFLARE_API_TOKEN=tok\n", "doesn't look like a domain"},
		{"spaces", "CARE_PUBLIC_HOST=my clinic.com\nCLOUDFLARE_API_TOKEN=tok\n", "doesn't look like a domain"},
		{"quoted token", "CARE_PUBLIC_HOST=clinic.example.com\nCLOUDFLARE_API_TOKEN=\"tok\"\n", "quotes, braces, or spaces"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tlsEngine(t, tc.body).ValidateTLS()
			if err == nil {
				t.Fatal("accepted an unusable config")
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Fatalf("unhelpful message: %v", err)
			}
		})
	}
}

// SaveTLSSettings must rewrite the values in place, keep the file's guidance
// comments, and refuse a bad pair rather than persist it.
func TestSaveTLSSettings(t *testing.T) {
	e := tlsEngine(t, "# guidance\nCARE_PUBLIC_HOST=old.example.com\nCLOUDFLARE_API_TOKEN=old\nCARE_ACME_CA=x\n")
	if err := e.SaveTLSSettings("new.example.com", "newtok"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(e.Kit, "tls.env"))
	got := string(b)
	for _, want := range []string{"# guidance", "CARE_PUBLIC_HOST=new.example.com", "CLOUDFLARE_API_TOKEN=newtok", "CARE_ACME_CA=x"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "old.example.com") || strings.Contains(got, "=old\n") {
		t.Fatalf("old values survived:\n%s", got)
	}
	// The file now holds a live credential.
	if fi, err := os.Stat(filepath.Join(e.Kit, "tls.env")); err != nil {
		t.Fatal(err)
	} else if fi.Mode().Perm() != 0o600 {
		t.Fatalf("tls.env mode %v, want 0600", fi.Mode().Perm())
	}

	if err := e.SaveTLSSettings("care.local", "tok"); err == nil {
		t.Fatal("saved a .local address")
	}
}

// The bundle URL must follow the live origin: a plain-http remoteEntry.js on an
// HTTPS page is blocked as mixed content and plugins vanish with no visible cause.
func TestBundleURLFollowsOrigin(t *testing.T) {
	e := tlsEngine(t, goodTLS)
	got := e.bundleURL(catalogEntry{Slug: "care_x", Bundle: "apps/care_x"})
	want := "https://clinic.example.com/facility-bucket/apps/care_x/assets/remoteEntry.js"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := e.bundlePath(catalogEntry{Slug: "care_x", Bundle: "apps/care_x"}); got != "/facility-bucket/apps/care_x" {
		t.Fatalf("bundlePath: %q", got)
	}
	custom := catalogEntry{Slug: "care_y", Bundle: "/apps/care_y/", RemoteEntry: "/remoteEntry.js"}
	if got := e.bundleURL(custom); got != "https://clinic.example.com/facility-bucket/apps/care_y/remoteEntry.js" {
		t.Fatalf("stray slashes not trimmed: %q", got)
	}
}

// Vite bakes the API URL in at build time, so changing the address must force a
// rebuild - otherwise the app keeps calling the old one and every request fails.
func TestFrontendRebuildsWhenOriginChanges(t *testing.T) {
	e := tlsEngine(t, goodTLS)
	marker := filepath.Join(e.Kit, frontendOriginFile)

	if e.frontendOriginCurrent() {
		t.Fatal("no marker: must rebuild rather than assume")
	}
	os.WriteFile(marker, []byte("https://old.example.com\n"), 0o644)
	if e.frontendOriginCurrent() {
		t.Fatal("stale origin should rebuild")
	}
	os.WriteFile(marker, []byte("https://clinic.example.com\n"), 0o644)
	if !e.frontendOriginCurrent() {
		t.Fatal("matching origin should not rebuild")
	}
}

func TestSetEnvValue(t *testing.T) {
	const src = "# comment\nREACT_CARE_API_URL=http://old\n# REACT_CARE_API_URL=commented\nOTHER=x\n"
	got := setEnvValue(src, "REACT_CARE_API_URL", "https://clinic.example.com")
	if !strings.Contains(got, "REACT_CARE_API_URL=https://clinic.example.com") {
		t.Fatalf("not replaced:\n%s", got)
	}
	if strings.Contains(got, "=http://old") {
		t.Fatalf("old value survived:\n%s", got)
	}
	if !strings.Contains(got, "# REACT_CARE_API_URL=commented") || !strings.Contains(got, "OTHER=x") {
		t.Fatalf("comments/other keys disturbed:\n%s", got)
	}
	if got := setEnvValue("A=1\n", "K", "v"); !strings.Contains(got, "K=v") {
		t.Fatalf("not appended: %q", got)
	}
}

// tls.env must resolve through the same chain as versions.env, and still lose to an
// explicit Env override.
func TestTLSSettingsResolution(t *testing.T) {
	e := tlsEngine(t, "CARE_PUBLIC_HOST=from-file.example.com\nCLOUDFLARE_API_TOKEN=tok\n")
	if got := e.publicHost(); got != "from-file.example.com" {
		t.Fatalf("tls.env not read: %q", got)
	}
	e.Env = map[string]string{"CARE_PUBLIC_HOST": "override.example.com"}
	if got := e.publicHost(); got != "override.example.com" {
		t.Fatalf("Env override not honored: %q", got)
	}
	if got := e.acmeCA(); got != acmeProduction {
		t.Fatalf("ACME default: %q", got)
	}
}

// HSTS must have a working default (an empty value would render as
// "max-age=" and be ignored by browsers) and still be overridable per clinic.
func TestHSTSSeconds(t *testing.T) {
	e := tlsEngine(t, goodTLS)
	if got := e.hstsSeconds(); got != "2592000" {
		t.Fatalf("default HSTS: %q", got)
	}
	e = tlsEngine(t, goodTLS+"CARE_HSTS_SECONDS=0\n")
	if got := e.hstsSeconds(); got != "0" {
		t.Fatalf("HSTS override: %q", got)
	}
}

// baseEnv is what compose reads; a setting missing here silently falls back to the
// compose default and the engine's value is ignored.
func TestBaseEnvCarriesTLSSettings(t *testing.T) {
	e := tlsEngine(t, goodTLS)
	env := map[string]string{}
	for _, kv := range e.baseEnv() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			env[k] = v // later entries win, matching exec's own precedence
		}
	}
	for k, want := range map[string]string{
		"CARE_PUBLIC_HOST":     "clinic.example.com",
		"CARE_PUBLIC_ORIGIN":   "https://clinic.example.com",
		"CLOUDFLARE_API_TOKEN": "tok",
		"CARE_ACME_CA":         acmeProduction,
		"CARE_HSTS_SECONDS":    "2592000",
	} {
		if env[k] != want {
			t.Fatalf("baseEnv[%s] = %q, want %q", k, env[k], want)
		}
	}
}

func TestZoneOf(t *testing.T) {
	for host, want := range map[string]string{
		"clinic.example.com":    "example.com",
		"a.b.example.co":        "example.co",
		"example.com":           "example.com",
		"clinic.didimakeit.com": "didimakeit.com",
	} {
		if got := zoneOf(host); got != want {
			t.Fatalf("zoneOf(%q) = %q, want %q", host, got, want)
		}
	}
}
