package care

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Setup is the one-time bootstrap: secret, backup dir, clone+build both images,
// then mDNS. Mirrors `care.sh setup`.
func (e *Engine) Setup() error {
	if err := e.genSecret(); err != nil {
		return err
	}
	if err := os.MkdirAll(e.backupDir(), 0o755); err != nil {
		return err
	}
	e.logln("Backups will go to: " + e.backupDir())
	if err := e.buildBackend(); err != nil {
		return err
	}
	if err := e.buildFrontend(); err != nil {
		return err
	}
	e.ensureMDNS()
	e.logln("Setup done.")
	return nil
}

// genSecret replaces DJANGO_SECRET_KEY=CHANGE_ME in backend.env with a random
// key. crypto/rand — strong, and no python/shell needed.
func (e *Engine) genSecret() error {
	path := filepath.Join(e.Kit, "backend.env")
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(b)
	if !strings.Contains(text, "DJANGO_SECRET_KEY=CHANGE_ME") {
		return nil
	}
	raw := make([]byte, 40)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	key := base64.RawURLEncoding.EncodeToString(raw) // ~54 url-safe chars
	text = strings.Replace(text, "DJANGO_SECRET_KEY=CHANGE_ME", "DJANGO_SECRET_KEY="+key, 1)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return err
	}
	e.logln("Generated a random DJANGO_SECRET_KEY in backend.env")
	return nil
}

func (e *Engine) clone(repo, ref, dir, label string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return nil // already cloned
	}
	e.logln("Cloning care " + label + " (" + ref + ") → " + dir)
	return e.run(nil, "git", "clone", "--depth", "1", "--branch", ref, repo, dir)
}

func (e *Engine) buildBackend() error {
	if err := e.clone(e.beRepo(), e.beRef(), e.beDir(), "backend"); err != nil {
		return err
	}
	e.logln("Building the backend image (" + e.backendImage() + ")... (several minutes)")
	df := filepath.Join(e.beDir(), "docker", "prod.Dockerfile")
	return e.run(nil, "docker", "build", "-f", df, "-t", e.backendImage(), e.beDir())
}

func (e *Engine) ensureBackendImage() error {
	if e.imageExists(e.backendImage()) {
		return nil
	}
	return e.buildBackend()
}

func (e *Engine) buildFrontend() error {
	if err := e.clone(e.feRepo(), e.feRef(), e.feDir(), "frontend"); err != nil {
		return err
	}
	// frontend.env overrides care_fe's committed .env (Vite reads .env.local).
	src, err := os.ReadFile(filepath.Join(e.Kit, "frontend.env"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(e.feDir(), ".env.local"), src, 0o644); err != nil {
		return err
	}
	e.logln("Building the frontend image (" + e.frontendImage() + ")... (a few minutes)")
	return e.run(nil, "docker", "build", "-t", e.frontendImage(), e.feDir())
}

func (e *Engine) ensureFrontendImage() error {
	if e.imageExists(e.frontendImage()) {
		return nil
	}
	return e.buildFrontend()
}

func (e *Engine) imageExists(tag string) bool {
	cmd := exec.Command("docker", "image", "inspect", tag)
	cmd.Env = e.baseEnv()
	return cmd.Run() == nil
}

// ensureMDNS makes http://<name>.local resolve on the LAN by renaming the machine.
// Only runs in "rename" mode now — the default "advertise" mode uses the in-app
// pure-Go responder (Advertise) instead, which needs no rename. Per-OS, best-effort:
// failures never abort setup (naming can be fixed by hand / static IP).
func (e *Engine) ensureMDNS() {
	if e.MDNSMode() != "rename" {
		return
	}
	name := e.mdnsName()
	switch runtime.GOOS {
	case "darwin":
		if cur, _ := e.capture("scutil", "--get", "LocalHostName"); cur == name {
			return
		}
		e.logln("Naming this Mac '" + name + "' so devices can use http://" + name + ".local ...")
		if err := e.run(nil, "sudo", "scutil", "--set", "LocalHostName", name); err != nil {
			e.logln("(skipped renaming — use the server IP)")
		}
	case "linux":
		if _, err := exec.LookPath("avahi-daemon"); err != nil {
			_ = e.run(nil, "sh", "-c", "sudo apt-get install -y avahi-daemon || sudo dnf install -y avahi || true")
		}
		_ = e.run(nil, "sudo", "hostnamectl", "set-hostname", name)
		_ = e.run(nil, "sudo", "systemctl", "enable", "--now", "avahi-daemon")
	case "windows":
		// Windows can't advertise <name>.local itself. One-time at setup: install
		// Apple Bonjour for a real care.local, or give the box a static IP.
		e.logln("Windows: set naming once — install Bonjour (for http://" + name + ".local) or use a static IP.")
	}
}
