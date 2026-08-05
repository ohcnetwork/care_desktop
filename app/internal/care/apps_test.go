package care

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A missing apps.json means "no optional apps offered", not a failure — the Apps
// tab must still open on a clinic that has no catalogue.
func TestLoadCatalogMissing(t *testing.T) {
	e := &Engine{Kit: t.TempDir()}
	entries, err := e.loadCatalog()
	if err != nil {
		t.Fatalf("missing apps.json should not be an error: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected no entries, got %v", entries)
	}
}

// A typo in the catalogue must be loud. Silently skipping the entry would look
// exactly like an app that was never listed.
func TestLoadCatalogRejectsBadEntries(t *testing.T) {
	for name, body := range map[string]string{
		"bad slug":   `{"apps":[{"slug":"../etc","bundle":"apps/x"}]}`,
		"no source":  `{"apps":[{"slug":"care_x","name":"X"}]}`,
		"bad json":   `{"apps":[`,
		"empty slug": `{"apps":[{"slug":"","bundle":"apps/x"}]}`,
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "apps.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		e := &Engine{Kit: dir}
		if _, err := e.loadCatalog(); err == nil {
			t.Fatalf("%s: expected an error", name)
		}
	}
}

// A bundle uploaded without an explicit content type arrives as
// application/octet-stream and the browser refuses to run it — the check that
// catches that has to read whichever shape the installed mc version prints.
func TestContentTypeOf(t *testing.T) {
	cases := map[string]string{
		`{"status":"success","contentType":"text/javascript"}`:                        "text/javascript",
		`{"status":"success","metadata":{"Content-Type":"text/javascript"}}`:          "text/javascript",
		`{"status":"success","metadata":{"content-type":"application/octet-stream"}}`: "application/octet-stream",
		"not json at all":      "",
		`{"status":"success"}`: "",
	}
	for raw, want := range cases {
		if got := contentTypeOf(raw); got != want {
			t.Fatalf("contentTypeOf(%s) = %q, want %q", raw, got, want)
		}
	}
	// mc may print several JSON objects; the useful one need not be first.
	multi := "{\"status\":\"success\"}\n{\"contentType\":\"text/javascript\"}"
	if got := contentTypeOf(multi); got != "text/javascript" {
		t.Fatalf("multi-line: %q", got)
	}
}

func TestIsJS(t *testing.T) {
	for _, ok := range []string{"text/javascript", "text/javascript; charset=utf-8", "APPLICATION/JAVASCRIPT"} {
		if !isJS(ok) {
			t.Fatalf("%q should be runnable", ok)
		}
	}
	for _, bad := range []string{"application/octet-stream", "text/plain", "", "application/json"} {
		if isJS(bad) {
			t.Fatalf("%q should not be runnable", bad)
		}
	}
}

// The panel must refuse to touch a row it did not create: switching off deletes
// the row, and a hand-made entry's settings could not be rebuilt afterwards.
func TestSetAppEnabledRefusesUnknownSlug(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "apps.json"), []byte(`{"apps":[{"slug":"care_x","bundle":"apps/care_x"}]}`), 0o644)
	e := &Engine{Kit: dir}
	err := e.SetAppEnabled("something_else", false)
	if err == nil {
		t.Fatal("expected a refusal for a slug outside the catalogue")
	}
	if !strings.Contains(err.Error(), "Apps page") {
		t.Fatalf("the error should point staff somewhere useful, got: %v", err)
	}
}
