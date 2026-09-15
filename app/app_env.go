package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/compose-spec/compose-go/v2/dotenv"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/atomicfile"
)

func (a *App) envPath(name string) (string, error) {
	switch name {
	case "backend":
		return filepath.Join(a.installDir(), "backend.env"), nil
	case "frontend":
		return filepath.Join(a.installDir(), "frontend.env"), nil
	}
	return "", errors.New("unknown env file: " + name)
}

func (a *App) ReadEnv(name, adminPassword string) (string, error) {
	var content string
	err := a.withJob(func() error {
		if err := a.requireAdmin(adminPassword); err != nil {
			return err
		}
		if err := a.requireSetup(); err != nil {
			return err
		}
		p, err := a.envPath(name)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		content = string(b)
		return err
	})
	return content, err
}

func (a *App) WriteEnv(name, content, adminPassword string) error {
	return a.withJob(func() error {
		if err := a.requireAdmin(adminPassword); err != nil {
			return err
		}
		if err := a.requireStableClinic(); err != nil {
			return err
		}
		p, err := a.envPath(name)
		if err != nil {
			return err
		}
		if _, err := dotenv.Parse(strings.NewReader(content)); err != nil {
			return errors.New("the environment file has invalid syntax; use KEY=value assignments and balanced quotes")
		}
		return atomicfile.Write(p, []byte(content), 0o600)
	})
}
