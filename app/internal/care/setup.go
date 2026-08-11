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
	if err := e.applyDomain(); err != nil {
		return err
	}
	if err := os.MkdirAll(e.backupDir(), 0o755); err != nil {
		return err
	}
	e.logln("Backups will go to: " + e.backupDir())
	// Backup image + keypair + WAF image, before the long clones (small build context).
	if err := e.ensureKeysDir(); err != nil {
		return err
	}
	if err := e.buildBackup(); err != nil {
		return err
	}
	if err := e.GenBackupKeypair(e.backupPassword()); err != nil {
		return err
	}
	if err := e.buildCaddy(); err != nil {
		return err
	}
	if err := e.buildBackend(); err != nil {
		return err
	}
	if err := e.buildFrontend(); err != nil {
		return err
	}
	e.ensureMDNS()
	// Hosts entry + cert trust wait for Start (ensureLocalAccess): one approval, and
	// the CA doesn't exist until Caddy runs.
	e.logln("Setup done.")
	return nil
}

// genSecret replaces DJANGO_SECRET_KEY=CHANGE_ME in backend.env with a random
// key. crypto/rand - strong, and no python/shell needed.
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
	e.logln("Cloning care " + label + " (" + ref + ") -> " + dir)
	return e.run(nil, "git", "clone", "--depth", "1", "--branch", ref, repo, dir)
}

func (e *Engine) buildBackend() error {
	if err := e.clone(e.beRepo(), e.beRef(), e.beDir(), "backend"); err != nil {
		return err
	}
	e.logln("Building the backend image (" + e.backendImage() + ")... (several minutes)")
	df := filepath.Join(e.beDir(), "docker", "prod.Dockerfile")
	args := []string{"build", "-f", df, "-t", e.backendImage()}
	// CARE pip-installs plugins at build time from ADDITIONAL_PLUGS.
	if plugs := e.additionalPlugs(); plugs != "" {
		e.logln("Building with plugins (ADDITIONAL_PLUGS set)")
		args = append(args, "--build-arg", "ADDITIONAL_PLUGS="+plugs)
	}
	args = append(args, e.beDir())
	return e.run(nil, "docker", args...)
}

func (e *Engine) ensureBackendImage() error {
	if e.imageExists(e.backendImage()) {
		return nil
	}
	return e.buildBackend()
}

// buildBackup builds the Postgres+openssl backup image. Runs before the clones land
// so the daemon isn't sent the (unused) hundreds of MB of source as build context.
func (e *Engine) buildBackup() error {
	e.logln("Building the backup image (" + e.backupImage() + ")...")
	df := filepath.Join(e.Kit, "backup.Dockerfile")
	return e.run(nil, "docker", "build", "-f", df,
		"--build-arg", "POSTGRES_IMAGE="+e.postgresImage(),
		"-t", e.backupImage(), e.Kit)
}

func (e *Engine) ensureBackupImage() error {
	if e.imageExists(e.backupImage()) {
		return nil
	}
	return e.buildBackup()
}

// buildCaddy builds the reverse-proxy image with the Coraza WAF compiled in (xcaddy).
func (e *Engine) buildCaddy() error {
	e.logln("Building the Caddy + WAF image (" + e.wafCaddyImage() + ")... (compiles Caddy; a few minutes)")
	df := filepath.Join(e.Kit, "caddy.Dockerfile")
	return e.run(nil, "docker", "build", "-f", df,
		"--build-arg", "CADDY_IMAGE="+e.caddyImage(),
		"-t", e.wafCaddyImage(), e.Kit)
}

func (e *Engine) ensureCaddyImage() error {
	if e.imageExists(e.wafCaddyImage()) {
		return nil
	}
	return e.buildCaddy()
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
	cmd := newCmd("docker", "image", "inspect", tag)
	cmd.Env = e.baseEnv()
	return cmd.Run() == nil
}

// ensureMDNS makes http://<name>.local resolve on the LAN by renaming the machine.
// Only runs in "rename" mode now - the default "advertise" mode uses the in-app
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
			e.logln("(skipped renaming - use the server IP)")
		}
	case "linux":
		if _, err := exec.LookPath("avahi-daemon"); err != nil {
			_ = e.run(nil, "sh", "-c", "sudo apt-get install -y avahi-daemon || sudo dnf install -y avahi || true")
		}
		_ = e.run(nil, "sudo", "hostnamectl", "set-hostname", name)
		_ = e.run(nil, "sudo", "systemctl", "enable", "--now", "avahi-daemon")
	case "windows":
		// This PC resolves <name>.local via the hosts entry (ensureLocalAccess).
		e.logln("Windows: this PC uses a hosts entry for https://" + name + ".local; other devices use mDNS or a static IP.")
	}
}
