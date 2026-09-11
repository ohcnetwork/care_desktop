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
	LogDir      string `json:"log_dir,omitempty"`       // diagnostic log location; empty means the platform default
	AdminPwHash string `json:"admin_pw_hash,omitempty"` // bcrypt of the install-time admin password; gates Advanced
}

// configPath and loadConfig are package-level: main opens the log before NewApp
// exists, and needs Config.LogDir to do it. Neither ever used the receiver.
func configPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir, _ = os.UserHomeDir()
	}
	dir = filepath.Join(dir, "care-desktop")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "config.json")
}

func loadConfig() Config {
	cfg := Config{MDNSName: "care.local"}
	b, err := os.ReadFile(configPath())
	if err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	if cfg.MDNSName == "" {
		cfg.MDNSName = "care.local"
	}
	return cfg
}

func (a *App) configPath() string { return configPath() }
func (a *App) loadConfig() Config { return loadConfig() }

func (a *App) saveConfig(cfg Config) error {
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(configPath(), b, 0o644)
}

// forgetConfig deletes the saved settings, so the next launch starts the wizard
// from scratch. Used by the purge; there is nothing worth keeping from an
// install whose data volumes have just been removed.
func (a *App) forgetConfig() {
	_ = os.Remove(configPath())
}
