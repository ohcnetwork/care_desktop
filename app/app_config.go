package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// --- persisted config -------------------------------------------------------

type Config struct {
	SetupDone   bool   `json:"setup_done"`
	MDNSName    string `json:"mdns_name"`
	InstallDir  string `json:"install_dir"`
	BackupDir   string `json:"backup_dir"`
	AdminPwHash string `json:"admin_pw_hash,omitempty"` // bcrypt of the install-time admin password; gates Advanced
}

func (a *App) configPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir, _ = os.UserHomeDir()
	}
	dir = filepath.Join(dir, "care-desktop")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "config.json")
}

func (a *App) loadConfig() Config {
	cfg := Config{MDNSName: "care.local"}
	b, err := os.ReadFile(a.configPath())
	if err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	if cfg.MDNSName == "" {
		cfg.MDNSName = "care.local"
	}
	return cfg
}

func (a *App) saveConfig(cfg Config) error {
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(a.configPath(), b, 0o644)
}

// forgetConfig deletes the saved settings, so the next launch starts the wizard
// from scratch. Used by the purge; there is nothing worth keeping from an
// install whose data volumes have just been removed.
func (a *App) forgetConfig() {
	_ = os.Remove(a.configPath())
}
