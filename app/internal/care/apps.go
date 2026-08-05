package care

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Optional CARE apps (frontend plugins).
//
// CARE loads frontend plugins at runtime from rows in its plug_config table, so
// switching one on or off is a database write — no image rebuild, nothing to
// download. That table is the ONLY record of on/off here: ListApps reads it fresh
// on every render and SetAppEnabled writes straight to it. It is the same row set
// CARE's own Apps admin page (/admin/apps) edits, and neither side keeps a copy,
// so the panel and that page cannot drift apart.
//
// apps.json is a catalogue, not state: it says which apps exist and where their
// bundle lives, never whether one is on.

// facilityBucket is the MinIO bucket minio/entrypoint.sh marks anonymous-public.
// Caddy proxies it at /facility-bucket/* on the app's own origin, so a plugin
// bundle copied there is reachable by every device on the LAN — offline, with no
// CORS and no extra service to run.
const facilityBucket = "facility-bucket"

// defaultRemoteEntry is where a care_fe plugin build leaves its federation entry.
const defaultRemoteEntry = "assets/remoteEntry.js"

// rowsPrefix tags the JSON line we ask Django to print, so stray output from the
// container (warnings, banners) can't be mistaken for the payload.
const rowsPrefix = "CARE_PLUGS_JSON:"

// slugRe gates slugs from apps.json and from the database before they reach a URL
// or a container command line.
var slugRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// ClinicApp is one row in the Apps tab.
type ClinicApp struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"` // a plug_config row exists → CARE loads it
	Managed     bool   `json:"managed"` // in the catalogue → the panel may toggle it
	Ready       bool   `json:"ready"`   // the bundle is installed and served correctly
	URL         string `json:"url"`     // where the browser fetches the plugin from
	Warning     string `json:"warning"` // plain-language problem, or ""
	NeedsPlug   string `json:"needs_backend_plug"`
}

type catalogEntry struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Bundle      string `json:"bundle"`
	URL         string `json:"url"`
	RemoteEntry string `json:"remote_entry"`
	NeedsPlug   string `json:"needs_backend_plug"`
}

// loadCatalog reads apps.json from the kit. A missing file means "no optional
// apps offered" and is not an error; a malformed one is, because a silent skip
// would look identical to an app that simply isn't listed.
func (e *Engine) loadCatalog() ([]catalogEntry, error) {
	b, err := os.ReadFile(filepath.Join(e.Kit, "apps.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var parsed struct {
		Apps []catalogEntry `json:"apps"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		return nil, fmt.Errorf("apps.json is not valid JSON: %w", err)
	}
	for _, entry := range parsed.Apps {
		if !slugRe.MatchString(entry.Slug) {
			return nil, fmt.Errorf("apps.json: %q is not a usable app slug (letters, digits, - and _ only)", entry.Slug)
		}
		if entry.Bundle == "" && entry.URL == "" {
			return nil, fmt.Errorf("apps.json: %q needs either a \"bundle\" or a \"url\"", entry.Slug)
		}
	}
	return parsed.Apps, nil
}

func remoteEntryOf(entry catalogEntry) string {
	if entry.RemoteEntry != "" {
		return strings.TrimLeft(entry.RemoteEntry, "/")
	}
	return defaultRemoteEntry
}

// bundleURL is the remoteEntry.js address for a bundle served out of the public
// facility bucket — same origin as the app itself, so no CORS. It must follow
// PublicOrigin rather than hardcoding http://<name>.local: on an HTTPS install a
// plain-http bundle URL is blocked as mixed content, and plugins would silently
// stop loading with nothing in the UI to explain why.
func (e *Engine) bundleURL(entry catalogEntry) string {
	return fmt.Sprintf("%s/%s/%s/%s",
		e.PublicOrigin(), facilityBucket, strings.Trim(entry.Bundle, "/"), remoteEntryOf(entry))
}

// bundlePath is the directory the plugin's own translations sit under. CARE reads
// meta.localPath verbatim for i18n and otherwise falls back to the URL's origin,
// which for a sub-path bundle would wrongly resolve to CARE's own /locale.
func (e *Engine) bundlePath(entry catalogEntry) string {
	return "/" + facilityBucket + "/" + strings.Trim(entry.Bundle, "/")
}

const listRowsPy = `import json
from care.users.models import PlugConfig
print("` + rowsPrefix + `" + json.dumps([{"slug": p.slug, "meta": p.meta} for p in PlugConfig.objects.all()]))`

const setRowPy = `import json, os
from django.core.cache import cache
from care.users.api.viewsets.plug_config import PlugConfigViewset
from care.users.models import PlugConfig

spec = json.loads(os.environ["CARE_APP_SPEC"])
if spec["on"]:
    PlugConfig.objects.update_or_create(slug=spec["slug"], defaults={"meta": spec["meta"]})
else:
    PlugConfig.objects.filter(slug=spec["slug"]).delete()

# The list endpoint caches with no expiry and is only invalidated by the viewset's
# own perform_* hooks, which a direct ORM write bypasses. Without this the Apps
# admin page and every browser keep serving the old list indefinitely.
cache.delete(PlugConfigViewset.cache_key)`

// plugRows returns the live plug_config rows keyed by slug — the single source of
// truth for which apps are on.
func (e *Engine) plugRows() (map[string]map[string]any, error) {
	out, err := e.capture("docker", "compose", "exec", "-T", "backend",
		"python", "manage.py", "shell", "-c", listRowsPy)
	if err != nil {
		return nil, fmt.Errorf("could not read the app list from CARE — is CARE running? (%w)", err)
	}
	for _, line := range strings.Split(out, "\n") {
		payload, ok := strings.CutPrefix(strings.TrimSpace(line), rowsPrefix)
		if !ok {
			continue
		}
		var rows []struct {
			Slug string         `json:"slug"`
			Meta map[string]any `json:"meta"`
		}
		if err := json.Unmarshal([]byte(payload), &rows); err != nil {
			return nil, fmt.Errorf("could not understand CARE's app list: %w", err)
		}
		byslug := make(map[string]map[string]any, len(rows))
		for _, r := range rows {
			byslug[r.Slug] = r.Meta
		}
		return byslug, nil
	}
	return nil, fmt.Errorf("unexpected output while reading CARE's app list: %s", out)
}

// contentTypeOf pulls the stored content-type out of `mc stat --json`, which has
// moved between top-level and metadata across mc versions. "" means unknown.
func contentTypeOf(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		var probe struct {
			ContentType string            `json:"contentType"`
			Metadata    map[string]string `json:"metadata"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(line)), &probe) != nil {
			continue
		}
		if probe.ContentType != "" {
			return probe.ContentType
		}
		for k, v := range probe.Metadata {
			if strings.EqualFold(k, "content-type") {
				return v
			}
		}
	}
	return ""
}

// isJS reports whether a browser will execute a module served with this type.
func isJS(contentType string) bool {
	base := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch base {
	case "text/javascript", "application/javascript", "application/x-javascript", "application/ecmascript", "text/ecmascript":
		return true
	}
	return false
}

// bundleReady checks the entry file is in the bucket and labelled as JavaScript.
// Anything it cannot determine counts as ready: a false alarm in front of clinic
// staff is worse than no alarm.
func (e *Engine) bundleReady(entry catalogEntry) (bool, string) {
	dir := strings.Trim(entry.Bundle, "/")
	object := fmt.Sprintf("local/%s/%s/%s", facilityBucket, dir, remoteEntryOf(entry))
	out, err := e.capture("docker", "compose", "exec", "-T", "minio", "mc", "stat", "--json", object)
	if err != nil {
		return false, "Not installed on this computer yet. Copy the app's build into " +
			facilityBucket + "/" + dir + " — see apps.json for the command."
	}
	if ct := contentTypeOf(out); ct != "" && !isJS(ct) {
		return false, "Installed, but the file is labelled \"" + ct + "\", which browsers refuse to run. " +
			"Copy it again with --attr \"Content-Type=text/javascript\"."
	}
	return true, ""
}

// ListApps merges the catalogue (what exists) with the live plug_config rows
// (what is on). Rows with no catalogue entry are included as read-only so the
// panel always shows the same set of apps as CARE's Apps admin page.
func (e *Engine) ListApps() ([]ClinicApp, error) {
	entries, err := e.loadCatalog()
	if err != nil {
		return nil, err
	}
	rows, err := e.plugRows()
	if err != nil {
		return nil, err
	}

	out := make([]ClinicApp, 0, len(entries)+len(rows))
	incatalog := make(map[string]bool, len(entries))

	for _, entry := range entries {
		incatalog[entry.Slug] = true

		app := ClinicApp{
			Slug:        entry.Slug,
			Name:        entry.Name,
			Description: entry.Description,
			Managed:     true,
			NeedsPlug:   entry.NeedsPlug,
		}
		if app.Name == "" {
			app.Name = entry.Slug
		}
		if entry.URL != "" {
			// Hosted elsewhere (a CDN) — there is nothing local to inspect.
			app.URL, app.Ready = entry.URL, true
		} else {
			app.URL = e.bundleURL(entry)
			app.Ready, app.Warning = e.bundleReady(entry)
		}

		meta, on := rows[entry.Slug]
		app.Enabled = on
		if on {
			// Someone may have edited this app's address on CARE's Apps page. Say so
			// rather than showing a plain "On" that hides the difference.
			if live, _ := meta["url"].(string); live != "" && live != app.URL {
				app.Warning = "On, but loading from " + live +
					" — set from CARE's Apps page. Switching it off and on again resets it."
			}
		}
		out = append(out, app)
	}

	for slug, meta := range rows {
		if incatalog[slug] {
			continue
		}
		url, _ := meta["url"].(string)
		out = append(out, ClinicApp{
			Slug:        slug,
			Name:        slug,
			Description: "Added by hand on CARE's Apps page.",
			Enabled:     true,
			Managed:     false,
			Ready:       true,
			URL:         url,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Managed != out[j].Managed {
			return out[i].Managed // catalogue apps first
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// SetAppEnabled switches a catalogue app on or off by writing CARE's plug_config
// table, then clearing the list cache so the change is live everywhere at once.
//
// It refuses slugs that aren't in the catalogue: switching off means deleting the
// row, and for a hand-made entry that would throw away settings the panel has no
// way to rebuild.
func (e *Engine) SetAppEnabled(slug string, enabled bool) error {
	entries, err := e.loadCatalog()
	if err != nil {
		return err
	}
	var entry catalogEntry
	for _, candidate := range entries {
		if candidate.Slug == slug {
			entry = candidate
			break
		}
	}
	if entry.Slug == "" {
		return fmt.Errorf("%q is not one of this clinic's listed apps — change it on CARE's Apps page instead, so its settings aren't lost", slug)
	}

	meta := map[string]any{"name": entry.Slug}
	if entry.URL != "" {
		meta["url"] = entry.URL
	} else {
		meta["url"] = e.bundleURL(entry)
		meta["localPath"] = e.bundlePath(entry)
	}

	spec, err := json.Marshal(map[string]any{"slug": entry.Slug, "on": enabled, "meta": meta})
	if err != nil {
		return err
	}

	name := entry.Name
	if name == "" {
		name = entry.Slug
	}
	e.logln(fmt.Sprintf("Turning %s %s...", name, map[bool]string{true: "on", false: "off"}[enabled]))

	// Passed through the environment, not interpolated into the script, so no
	// quoting in a name or URL can change what Python runs.
	if err := e.dc("exec", "-T", "-e", "CARE_APP_SPEC="+string(spec),
		"backend", "python", "manage.py", "shell", "-c", setRowPy); err != nil {
		return fmt.Errorf("could not update the app list — CARE must be running to change apps (%w)", err)
	}

	e.logln("Done. Staff need to refresh CARE in their browser to see the change.")
	return nil
}
