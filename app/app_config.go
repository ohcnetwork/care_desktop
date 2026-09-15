package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/atomicfile"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/mdns"
)

const appDirName = "care-desktop"

type Config struct {
	SetupDone   bool   `json:"setup_done"`
	Removing    bool   `json:"removing,omitempty"`
	MDNSName    string `json:"mdns_name"`
	BackupDir   string `json:"backup_dir"`
	AdminPwHash string `json:"admin_pw_hash"`
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir, err = os.UserHomeDir()
	}
	if err != nil {
		return "", fmt.Errorf("could not find the settings directory: %w", err)
	}
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("the settings directory must be an absolute path: %q", dir)
	}
	return filepath.Join(dir, appDirName, "config.json"), nil
}

func loadConfig(path string) (Config, error) {
	var cfg *Config
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("could not read saved settings at %s: %w", path, err)
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("saved settings at %s are damaged; preserve this file and recover the settings before continuing: %w", path, err)
	}
	if cfg == nil {
		return Config{}, fmt.Errorf("saved settings at %s must be a JSON object", path)
	}
	if cfg.BackupDir != "" && !filepath.IsAbs(cfg.BackupDir) {
		return Config{}, fmt.Errorf("the saved backup directory must be an absolute path: %s", cfg.BackupDir)
	}
	if cfg.MDNSName != "" {
		if err := mdns.ValidateLabel(cfg.MDNSName); err != nil {
			return Config{}, fmt.Errorf("the saved clinic address is invalid: %w", err)
		}
	}
	return *cfg, nil
}

func (a *App) configPath() string { return a.configFile }

func (a *App) loadConfig() Config {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg
}

func (a *App) saveConfig(cfg Config) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicfile.Write(a.configPath(), b, 0o600); err != nil {
		return fmt.Errorf("could not save settings: %w", err)
	}
	a.cfg = cfg
	return nil
}

func (a *App) forgetConfig() error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if err := os.Remove(a.configPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not remove saved settings: %w", err)
	}
	a.cfg = Config{}
	return nil
}
