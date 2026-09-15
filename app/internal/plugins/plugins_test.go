package plugins

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPluginEnvironmentRoundTrip(t *testing.T) {
	t.Setenv("CARE_PLUGIN_VARIABLE", "must-not-expand")
	dir := t.TempDir()
	m := New(dir)
	if _, err := m.ReadPlugins(); err == nil {
		t.Fatal("a missing environment file became an empty plugin list")
	}
	path := filepath.Join(dir, "backend.env")
	if err := os.WriteFile(path, []byte("OTHER=value\nADDITIONAL_PLUGS=[]\nexport ADDITIONAL_PLUGS=[]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := []Plugin{{Name: "example", PackageName: "example", Configs: map[string]any{
		"value": "$CARE_PLUGIN_VARIABLE and O'Connor\\path",
	}}}
	if err := m.WritePlugins(want); err != nil {
		t.Fatal(err)
	}
	got, err := m.ReadPlugins()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("plugin values changed: %#v, %v", got, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "OTHER=value") || strings.Count(string(data), additionalPlugsKey+"=") != 1 {
		t.Fatalf("environment editing changed unrelated settings or left duplicates: %s, %v", data, err)
	}
	if err := m.WritePlugins(nil); err != nil {
		t.Fatal(err)
	}
	if got, err := m.ReadPlugins(); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("plugins were not removed: %#v, %v", got, err)
	}
}
