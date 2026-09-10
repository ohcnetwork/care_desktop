package clinic

import (
	"github.com/ohcnetwork/care_desktop/app/internal/plugins"
)

// Plugins binds a plugin manager to this engine's install dir and runner.
func (e *Clinic) Plugins() *plugins.Manager {
	return plugins.New(e.Runner(), e.InstallDir, e.Log)
}
