package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Manager struct {
	Dir string
}

func New(dir string) *Manager { return &Manager{Dir: dir} }

const additionalPlugsKey = "ADDITIONAL_PLUGS"

type Plugin struct {
	Name        string         `json:"name"`
	PackageName string         `json:"package_name"`
	Version     string         `json:"version,omitempty"`
	Configs     map[string]any `json:"configs,omitempty"`
}

func (m *Manager) backendEnvPath() string { return filepath.Join(m.Dir, "backend.env") }

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
