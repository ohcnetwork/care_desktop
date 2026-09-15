package plugins

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/compose-spec/compose-go/v2/dotenv"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/atomicfile"
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
	raw, err := m.AdditionalPlugs()
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return []Plugin{}, nil
	}
	plugs := []Plugin{}
	if err := json.Unmarshal([]byte(raw), &plugs); err != nil {
		return nil, err
	}
	if plugs == nil {
		return []Plugin{}, nil
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
		raw = "'" + strings.ReplaceAll(string(b), "'", `\'`) + "'"
	}
	return m.setEnvVar(additionalPlugsKey, raw)
}

func (m *Manager) AdditionalPlugs() (string, error) {
	value, err := m.envVar(additionalPlugsKey)
	return strings.TrimSpace(value), err
}

func (m *Manager) envVar(key string) (string, error) {
	b, err := os.ReadFile(m.backendEnvPath())
	if err != nil {
		return "", err
	}
	env, err := dotenv.Parse(bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	return env[key], nil
}

func (m *Manager) setEnvVar(key, val string) error {
	path := m.backendEnvPath()
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := make([]string, 0)
	found := false
	for _, line := range strings.Split(string(b), "\n") {
		text := strings.TrimPrefix(strings.TrimSpace(line), "export ")
		name, _, ok := strings.Cut(text, "=")
		if ok && strings.TrimSpace(name) == key {
			if !found && val != "" {
				lines = append(lines, key+"="+val)
			}
			found = true
			continue
		}
		lines = append(lines, line)
	}
	if !found && val != "" {
		if n := len(lines); n > 0 && lines[n-1] == "" {
			lines = append(lines[:n-1], key+"="+val, "")
		} else {
			lines = append(lines, key+"="+val)
		}
	}
	output := strings.Join(lines, "\n")
	if _, err := dotenv.Parse(strings.NewReader(output)); err != nil {
		return err
	}
	return atomicfile.Write(path, []byte(output), 0o600)
}
