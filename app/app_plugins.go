package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ohcnetwork/care_desktop/app/internal/plugins"
)

func (a *App) ReadPlugins() ([]plugins.Plugin, error) {
	if _, err := os.Stat(filepath.Join(a.installDir(), "backend.env")); err != nil {
		return []plugins.Plugin{}, nil // not set up yet
	}
	return plugins.New(a.installDir()).ReadPlugins()
}

func (a *App) SavePlugins(pluginList []plugins.Plugin) error {
	if _, err := os.Stat(filepath.Join(a.installDir(), "backend.env")); err != nil {
		return errors.New("not set up yet - run the first-time setup")
	}
	return plugins.New(a.installDir()).WritePlugins(pluginList)
}
