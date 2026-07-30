package care

import (
	"encoding/json"
	"fmt"
	"sort"
)

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
func (e *Engine) ReadFrontendPlugins() ([]FrontendPlugin, error) {
	rows, err := e.plugRows()
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
func (e *Engine) WriteFrontendPlugins(plugs []FrontendPlugin) error {
	for _, p := range plugs {
		if !slugRe.MatchString(p.Slug) {
			return fmt.Errorf("%q is not a usable plugin name (letters, digits, - and _ only)", p.Slug)
		}
	}
	spec, err := json.Marshal(plugs)
	if err != nil {
		return err
	}
	e.logln("Saving frontend plugins...")
	if err := e.dc("exec", "-T", "-e", "CARE_FE_PLUGS="+string(spec),
		"backend", "python", "manage.py", "shell", "-c", setFePlugsPy); err != nil {
		return fmt.Errorf("could not save frontend plugins — CARE must be running to change them (%w)", err)
	}
	e.logln("Done. Staff refresh CARE in their browser to see the change.")
	return nil
}
