package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ohcnetwork/care_desktop/app/internal/plugins"
)

// --- backend plugins (ADDITIONAL_PLUGS in backend.env) ----------------------

// ReadPlugins returns the backend plugins configured for this install.
func (a *App) ReadPlugins() ([]plugins.Plugin, error) {
	if _, err := os.Stat(filepath.Join(a.installDir(), "backend.env")); err != nil {
		return []plugins.Plugin{}, nil // not set up yet
	}
	return a.engine().Plugins().ReadPlugins()
}

// SavePlugins writes the plugin list; the UI follows with a rebuild-backend.
func (a *App) SavePlugins(plugins []plugins.Plugin) error {
	if _, err := os.Stat(filepath.Join(a.installDir(), "backend.env")); err != nil {
		return errors.New("not set up yet - run the first-time setup")
	}
	return a.engine().Plugins().WritePlugins(plugins)
}
