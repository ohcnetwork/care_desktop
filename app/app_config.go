package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const appDirName = "care-desktop"

type Config struct {
	SetupDone   bool   `json:"setup_done"`
	MDNSName    string `json:"mdns_name"`
	BackupDir   string `json:"backup_dir"`
	AdminPwHash string `json:"admin_pw_hash"`
}

func configPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir, _ = os.UserHomeDir()
	}
	dir = filepath.Join(dir, appDirName)
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "config.json")
}

func loadConfig() Config {
	var cfg Config
	if b, err := os.ReadFile(configPath()); err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	return cfg
}

func (a *App) configPath() string { return configPath() }
func (a *App) loadConfig() Config { return loadConfig() }

func (a *App) saveConfig(cfg Config) error {
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(configPath(), b, 0o644)
}

func (a *App) forgetConfig() {
	p := configPath()
	_ = os.Remove(p)
	_ = os.Remove(filepath.Dir(p))
}
