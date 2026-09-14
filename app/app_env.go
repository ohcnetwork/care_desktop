package main

import (
	"errors"
	"os"
	"path/filepath"
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

func (a *App) ReadEnv(name string) (string, error) {
	p, err := a.envPath(name)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	return string(b), err
}

func (a *App) WriteEnv(name, content string) error {
	p, err := a.envPath(name)
	if err != nil {
		return err
	}
	return os.WriteFile(p, []byte(content), 0o644)
}
