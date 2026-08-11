package care

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateMDNSLabel(t *testing.T) {
	ok := []string{"care", "caretest", "care-test", "care.local", "CARE", "clinic2"}
	for _, n := range ok {
		if err := ValidateMDNSLabel(n); err != nil {
			t.Errorf("%q should be accepted: %v", n, err)
		}
	}
	bad := []string{"", "   ", "care local", "care_test", "-care", "care-", "café", strings.Repeat("a", 64)}
	for _, n := range bad {
		if err := ValidateMDNSLabel(n); err == nil {
			t.Errorf("%q should be rejected", n)
		}
	}
}

// localhost and the pki/authorities/local path must survive: rewriting either
// breaks the health probe or the cert bootstrap.
func TestHostReLeavesNonHostsAlone(t *testing.T) {
	in := "localhost:443 {\n  root * /data/caddy/pki/authorities/local\n  care.local:443\n}"
	out := hostRe.ReplaceAllString(in, "caretest.local")
	if !strings.Contains(out, "localhost:443") {
		t.Error("localhost was rewritten")
	}
	if !strings.Contains(out, "authorities/local\n") {
		t.Error("the pki path was rewritten")
	}
	if !strings.Contains(out, "caretest.local:443") {
		t.Error("the site address was not rewritten")
	}
}

func TestApplyDomainRewritesEveryFile(t *testing.T) {
	kit := t.TempDir()
	if err := os.MkdirAll(filepath.Join(kit, "setup"), 0o755); err != nil {
		t.Fatal(err)
	}
	seed := map[string]string{
		"Caddyfile":        "care.local:443 {\n  tls internal\n}\nlocalhost:443 {\n}",
		"backend.env":      "BUCKET_EXTERNAL_ENDPOINT=https://care.local\nCSRF_TRUSTED_ORIGINS=[\"https://care.local\"]\nPOSTGRES_HOST=db\n",
		"frontend.env":     "REACT_CARE_API_URL=https://care.local\n",
		"setup/index.html": "<p>open https://care.local/setup</p>",
	}
	for rel, body := range seed {
		if err := os.WriteFile(filepath.Join(kit, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	e := &Engine{Kit: kit, Env: map[string]string{"CARE_MDNS_NAME": "caretest"}}
	if err := e.applyDomain(); err != nil {
		t.Fatal(err)
	}
	for rel := range seed {
		b, err := os.ReadFile(filepath.Join(kit, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		got := string(b)
		if strings.Contains(got, "care.local") && !strings.Contains(got, "caretest.local") {
			t.Errorf("%s still points at the old address:\n%s", rel, got)
		}
		if !strings.Contains(got, "caretest.local") {
			t.Errorf("%s was not rewritten:\n%s", rel, got)
		}
	}
	// Changing it again must land, even though "care.local" is now gone.
	e.Env["CARE_MDNS_NAME"] = "clinic2"
	if err := e.applyDomain(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(kit, "frontend.env"))
	if !strings.Contains(string(b), "https://clinic2.local") {
		t.Errorf("second rename did not land: %s", b)
	}
	// Unrelated settings must be untouched.
	b, _ = os.ReadFile(filepath.Join(kit, "backend.env"))
	if !strings.Contains(string(b), "POSTGRES_HOST=db") {
		t.Error("applyDomain touched an unrelated setting")
	}
}

// host() is the only normaliser. "care.local" must not become care.local.local
// (the hosts entry and the dialog would then name an address nothing serves), and
// case must be flattened so the baked API URL matches what a browser sends.
func TestHostNormalisesInput(t *testing.T) {
	for in, want := range map[string]string{
		"care":            "care.local",
		"care.local":      "care.local",
		"CARE":            "care.local",
		"CARE.LOCAL":      "care.local",
		"  caretest  ":    "caretest.local",
		"caretest.local.": "caretest.local",
	} {
		e := &Engine{Env: map[string]string{"CARE_MDNS_NAME": in}}
		if got := e.host(); got != want {
			t.Errorf("host(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestApplyDomainNormalisesCase(t *testing.T) {
	kit := t.TempDir()
	if err := os.WriteFile(filepath.Join(kit, "frontend.env"),
		[]byte("REACT_CARE_API_URL=https://care.local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := &Engine{Kit: kit, Env: map[string]string{"CARE_MDNS_NAME": "CareTest"}}
	if err := e.applyDomain(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(kit, "frontend.env"))
	if !strings.Contains(string(b), "https://caretest.local") {
		t.Errorf("address was not lowercased: %s", b)
	}
}

func TestApplyDomainRejectsBadName(t *testing.T) {
	e := &Engine{Kit: t.TempDir(), Env: map[string]string{"CARE_MDNS_NAME": "care test"}}
	if err := e.applyDomain(); err == nil {
		t.Fatal("a name with a space should be rejected before anything is written")
	}
}

func TestWarnDomainDrift(t *testing.T) {
	kit := t.TempDir()
	if err := os.WriteFile(filepath.Join(kit, "Caddyfile"),
		[]byte("care.local:443 {\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	logs := func(name string) []string {
		var out []string
		e := &Engine{Kit: kit, Env: map[string]string{"CARE_MDNS_NAME": name},
			Log: func(s string) { out = append(out, s) }}
		e.warnDomainDrift()
		return out
	}
	if got := logs("caretest"); len(got) != 1 || !strings.Contains(got[0], "care.local") {
		t.Errorf("a renamed install must warn, got %v", got)
	}
	if got := logs("care"); len(got) != 0 {
		t.Errorf("a matching install must stay quiet, got %v", got)
	}
}
