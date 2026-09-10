// Package plugins reads and writes the backend and frontend plugin lists that
// live in the install directory's env files. See docs/plugins.md.
package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

// Manager edits the plugin configuration for one install directory.
type Manager struct {
	Dir string       // install dir, holding backend.env and frontend.env
	Log func(string) // optional line sink

	run proc.Runner
}

// New binds a Manager to a command runner.
func New(run proc.Runner, dir string, log func(string)) *Manager {
	return &Manager{Dir: dir, Log: log, run: run}
}

func (m *Manager) logln(s string) {
	if m.Log != nil {
		m.Log(s)
	}
}

func (m *Manager) dc(args ...string) error {
	return m.run.Run("docker", append([]string{"compose"}, args...)...)
}

// ADDITIONAL_PLUGS (in backend.env) is CARE's plugin list. It's read at build time
// (pip-installs packages) AND runtime, so enabling a plugin needs a backend rebuild,
// not a restart - buildBackend passes it as a --build-arg.
const additionalPlugsKey = "ADDITIONAL_PLUGS"

// Plugin mirrors CARE's Plug dataclass. Configs is map[string]any so values keep
// their JSON type (a boolean false actually disables a plugin flag; a string "false"
// would be truthy in Python). omitempty: blank version -> CARE's default, empty configs dropped.
type Plugin struct {
	Name        string         `json:"name"`
	PackageName string         `json:"package_name"`
	Version     string         `json:"version,omitempty"`
	Configs     map[string]any `json:"configs,omitempty"`
}

func (m *Manager) backendEnvPath() string { return filepath.Join(m.Dir, "backend.env") }

// ReadPlugins parses the plugin list out of ADDITIONAL_PLUGS in backend.env. A
// missing or empty var means no plugins (not an error).
func (m *Manager) ReadPlugins() ([]Plugin, error) {
	raw := strings.TrimSpace(m.envVar(additionalPlugsKey))
	if raw == "" {
		return []Plugin{}, nil
	}
	var plugs []Plugin
	if err := json.Unmarshal([]byte(raw), &plugs); err != nil {
		return nil, err
	}
	return plugs, nil
}

// WritePlugins writes the list to ADDITIONAL_PLUGS (empty list removes the var).
func (m *Manager) WritePlugins(plugs []Plugin) error {
	raw := ""
	if len(plugs) > 0 {
		b, err := json.Marshal(plugs)
		if err != nil {
			return err
		}
		raw = string(b)
	}
	return m.setEnvVar(additionalPlugsKey, raw)
}

// AdditionalPlugs is the raw ADDITIONAL_PLUGS value from backend.env.
func (m *Manager) AdditionalPlugs() string { return strings.TrimSpace(m.envVar(additionalPlugsKey)) }

func (m *Manager) envVar(key string) string {
	b, err := os.ReadFile(m.backendEnvPath())
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), key+"="); ok {
			return v
		}
	}
	return ""
}

// setEnvVar upserts key=val in backend.env, preserving other lines (empty val removes it).
func (m *Manager) setEnvVar(key, val string) error {
	path := m.backendEnvPath()
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), key+"=") {
			if val == "" {
				lines = append(lines[:i], lines[i+1:]...)
			} else {
				lines[i] = key + "=" + val
			}
			return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
		}
	}
	if val != "" {
		if n := len(lines); n > 0 && lines[n-1] == "" {
			lines = append(lines[:n-1], key+"="+val, "")
		} else {
			lines = append(lines, key+"="+val)
		}
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

// Frontend plugins, the CARE-native way.
//
// CARE loads frontend plugins at runtime from rows in its plug_config table — the
// same rows CARE's own Apps admin page (/admin/apps) edits, keyed by slug with a
// free-form meta blob (meta.url is the module-federation remoteEntry.js the browser
// loads). Adding, editing, or removing one is a database write, so there is nothing
// to build and nothing to download: the change is live as soon as staff refresh.
//
// This mirrors CARE's admin form (a slug + a meta JSON), which the desktop panel
// then dresses up with proper fields. plugRows/rowsPrefix/slugRe/cache handling all
// live in apps.go and are shared.

// FrontendPlugin is one plug_config row. Meta is carried whole — url, name, config
// and any other keys — so editing a plugin never silently drops metadata the panel
// doesn't render.
type FrontendPlugin struct {
	Slug string         `json:"slug"`
	Meta map[string]any `json:"meta"`
}

// setFePlugsPy reconciles plug_config to exactly the list it's given: every plugin
// is written, any row not in the list is deleted, and the list cache is cleared so
// the change is live at once. Passed through the environment, never interpolated,
// so a quote in a name or URL can't change what Python runs.
const setFePlugsPy = `import json, os
from django.core.cache import cache
from care.users.api.viewsets.plug_config import PlugConfigViewset
from care.users.models import PlugConfig

desired = json.loads(os.environ["CARE_FE_PLUGS"]) or []
slugs = []
for item in desired:
    slugs.append(item["slug"])
    PlugConfig.objects.update_or_create(slug=item["slug"], defaults={"meta": item.get("meta") or {}})
PlugConfig.objects.exclude(slug__in=slugs).delete()

# The list endpoint caches with no expiry and is only invalidated by the viewset's
# own perform_* hooks, which a direct ORM write bypasses.
cache.delete(PlugConfigViewset.cache_key)`

// ReadFrontendPlugins returns CARE's live plug_config rows — the frontend plugins
// that are on right now, the same set CARE's own Apps page shows.
func (m *Manager) ReadFrontendPlugins() ([]FrontendPlugin, error) {
	rows, err := m.plugRows()
	if err != nil {
		return nil, err
	}
	out := make([]FrontendPlugin, 0, len(rows))
	for slug, meta := range rows {
		if meta == nil {
			meta = map[string]any{}
		}
		out = append(out, FrontendPlugin{Slug: slug, Meta: meta})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

// WriteFrontendPlugins makes CARE's plug_config table match the given list exactly
// (add + edit + remove in one write), then clears the list cache. No rebuild — CARE
// loads plugins at runtime. The caller is expected to have loaded the current rows
// first, so a plugin missing from the list means "the user removed it", not "unknown".
func (m *Manager) WriteFrontendPlugins(plugs []FrontendPlugin) error {
	for _, p := range plugs {
		if !slugRe.MatchString(p.Slug) {
			return fmt.Errorf("%q is not a usable plugin name (letters, digits, - and _ only)", p.Slug)
		}
	}
	spec, err := json.Marshal(plugs)
	if err != nil {
		return err
	}
	m.logln("Saving frontend plugins...")
	if err := m.dc("exec", "-T", "-e", "CARE_FE_PLUGS="+string(spec),
		"backend", "python", "manage.py", "shell", "-c", setFePlugsPy); err != nil {
		return fmt.Errorf("could not save frontend plugins — CARE must be running to change them (%w)", err)
	}
	m.logln("Done. Staff refresh CARE in their browser to see the change.")
	return nil
}

const rowsPrefix = "CARE_PLUGS_JSON:"

// slugRe gates slugs before they reach a URL or a container command line.
var slugRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

const listRowsPy = `import json
from care.users.models import PlugConfig
print("` + rowsPrefix + `" + json.dumps([{"slug": p.slug, "meta": p.meta} for p in PlugConfig.objects.all()]))`

func (m *Manager) plugRows() (map[string]map[string]any, error) {
	out, err := m.run.Capture("docker", "compose", "exec", "-T", "backend",
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
