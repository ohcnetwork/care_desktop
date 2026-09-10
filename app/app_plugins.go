package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/ohcnetwork/care_desktop/app/internal/clinic"
	"github.com/ohcnetwork/care_desktop/app/internal/plugins"
)

// --- backend plugins (ADDITIONAL_PLUGS in backend.env) ----------------------

// ReadPlugins returns the backend plugins configured for this install.
func (a *App) ReadPlugins() ([]plugins.Plugin, error) {
	if _, err := os.Stat(filepath.Join(a.installDir(), "backend.env")); err != nil {
		return []plugins.Plugin{}, nil // not set up yet
	}
	return a.engine(nil).Plugins().ReadPlugins()
}

// SavePlugins writes the plugin list; the UI follows with a rebuild-backend.
func (a *App) SavePlugins(plugins []plugins.Plugin) error {
	if _, err := os.Stat(filepath.Join(a.installDir(), "backend.env")); err != nil {
		return errors.New("not set up yet - run the first-time setup")
	}
	return a.engine(nil).Plugins().WritePlugins(plugins)
}

// --- frontend plugins (CARE plug_config table, synced with /admin/apps) ------
//
// Unlike backend plugins, frontend plugins load at runtime from CARE's plug_config
// table, so a toggle is a single database write - no rebuild. The engine reads and
// writes those rows directly, the same ones CARE's own Apps page edits, so the two
// panels stay in sync.

// appsEngine pins the clinic's chosen mDNS name so plugin URLs resolve; engine(nil)
// would fall back to "care".
func (a *App) appsEngine() *clinic.Clinic {
	host := strings.TrimSuffix(strings.TrimSpace(a.loadConfig().MDNSName), ".local")
	if host == "" {
		host = "care"
	}
	return a.engine(map[string]string{"CARE_MDNS_NAME": host})
}

func (a *App) ReadFrontendPlugins() ([]plugins.FrontendPlugin, error) {
	if _, err := os.Stat(filepath.Join(a.installDir(), "docker-compose.yml")); err != nil {
		return []plugins.FrontendPlugin{}, nil // not set up yet
	}
	return a.appsEngine().Plugins().ReadFrontendPlugins()
}

// SaveFrontendPlugins writes the whole plugin list to CARE's plug_config table
// (add + edit + remove). Instant - no rebuild, since CARE loads them at runtime.
func (a *App) SaveFrontendPlugins(plugins []plugins.FrontendPlugin) error {
	if _, err := os.Stat(filepath.Join(a.installDir(), "docker-compose.yml")); err != nil {
		return errors.New("not set up yet - run the first-time setup")
	}
	return a.appsEngine().Plugins().WriteFrontendPlugins(plugins)
}
