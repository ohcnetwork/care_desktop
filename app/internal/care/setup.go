package care

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Setup is the one-time bootstrap: secret, backup dir, clone+build every image.
func (e *Engine) Setup() error {
	// Before the ~10-minute builds: a bad HTTPS setting is otherwise only discovered
	// at the very end, after the whole wait.
	if err := e.ValidateTLS(); err != nil {
		return err
	}
	if err := e.genSecret(); err != nil {
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

// frontendOriginFile records the origin the current frontend image was built for.
// Vite bakes REACT_CARE_API_URL in at build time, so changing the clinic's public
// name needs a rebuild, not a restart. Without this marker the running app would keep
// calling the old origin and every API request would fail as cross-origin - a
// confusing half-broken state.
const frontendOriginFile = ".frontend-origin"

func (e *Engine) buildFrontend() error {
	if err := e.clone(e.feRepo(), e.feRef(), e.feDir(), "frontend"); err != nil {
		return err
	}
	// frontend.env overrides care_fe's committed .env (Vite reads .env.local). The
	// API URL is forced to the live origin so it can't drift from what Caddy serves.
	src, err := os.ReadFile(filepath.Join(e.Kit, "frontend.env"))
	if err != nil {
		return err
	}
	origin := e.PublicOrigin()
	text := setEnvValue(string(src), "REACT_CARE_API_URL", origin)
	if err := os.WriteFile(filepath.Join(e.feDir(), ".env.local"), []byte(text), 0o644); err != nil {
		return err
	}
	e.logln("Building the frontend image (" + e.frontendImage() + ") for " + origin + "... (a few minutes)")
	if err := e.run(nil, "docker", "build", "-t", e.frontendImage(), e.feDir()); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(e.Kit, frontendOriginFile), []byte(origin), 0o644)
}

func (e *Engine) ensureFrontendImage() error {
	if e.imageExists(e.frontendImage()) && e.frontendOriginCurrent() {
		return nil
	}
	return e.buildFrontend()
}

// frontendOriginCurrent reports whether the built image already targets the origin
// we're about to serve. A missing marker means we can't know what it was built for,
// so rebuild rather than risk shipping an app that calls the wrong address.
func (e *Engine) frontendOriginCurrent() bool {
	b, err := os.ReadFile(filepath.Join(e.Kit, frontendOriginFile))
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(b)) == e.PublicOrigin()
}

// setEnvValue replaces KEY=... in a dotenv blob, or appends it if absent, leaving
// comments and ordering untouched. Commented-out lines are left alone.
func setEnvValue(text, key, value string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), key+"=") {
			lines[i] = key + "=" + value
			return strings.Join(lines, "\n")
		}
	}
	return strings.TrimRight(text, "\n") + "\n" + key + "=" + value + "\n"
}

func (e *Engine) imageExists(tag string) bool {
	cmd := exec.Command("docker", "image", "inspect", tag)
	cmd.Env = e.baseEnv()
	return cmd.Run() == nil
}
