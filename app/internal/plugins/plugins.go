// Package plugins reads and writes the backend plugin list (ADDITIONAL_PLUGS in
// backend.env). Frontend plugins are managed in CARE's own admin pages.
package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Manager edits the plugin configuration for one install directory.
type Manager struct {
	Dir string // install dir, holding backend.env
}

func New(dir string) *Manager { return &Manager{Dir: dir} }

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
