package care

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Plugins must round-trip through ADDITIONAL_PLUGS in backend.env without disturbing
// the other lines, and an empty list must remove the var.
func TestPluginsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	be := filepath.Join(dir, "backend.env")
	orig := "POSTGRES_USER=postgres\nDATABASE_URL=postgres://x\n"
	if err := os.WriteFile(be, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	e := &Engine{Kit: dir}

	// empty to start
	if got, _ := e.ReadPlugins(); len(got) != 0 {
		t.Fatalf("want 0 plugins, got %d", len(got))
	}

	plugs := []Plugin{
		{Name: "hcx", PackageName: "git+https://github.com/ohcnetwork/care_hcx.git", Version: "@master",
			Configs: map[string]any{"HCX_URL": "https://x"}},
		{Name: "abdm", PackageName: "care_abdm"}, // no version -> omitted -> CARE default
	}
	if err := e.WritePlugins(plugs); err != nil {
		t.Fatal(err)
	}

	// other lines survive
	b, _ := os.ReadFile(be)
	if !strings.Contains(string(b), "POSTGRES_USER=postgres") || !strings.Contains(string(b), "DATABASE_URL=postgres://x") {
		t.Fatalf("existing env lines lost:\n%s", b)
	}
	// blank version is omitted from the JSON
	if strings.Contains(string(b), `"version":""`) {
		t.Fatalf("blank version should be omitted:\n%s", b)
	}

	got, err := e.ReadPlugins()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "hcx" || got[0].Configs["HCX_URL"] != "https://x" || got[1].Name != "abdm" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}

	// build arg value is the compact JSON
	if raw := e.additionalPlugs(); !strings.HasPrefix(raw, "[{") || !strings.Contains(raw, `"care_abdm"`) {
		t.Fatalf("additionalPlugs raw wrong: %q", raw)
	}

	// empty list removes the var
	if err := e.WritePlugins(nil); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(be); strings.Contains(string(b), additionalPlugsKey) {
		t.Fatalf("ADDITIONAL_PLUGS not removed:\n%s", b)
	}
}
