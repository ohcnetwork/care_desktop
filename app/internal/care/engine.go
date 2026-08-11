// Package care is the cross-platform engine that drives the CARE clinic stack.
// It replaces the old care.sh: every action is plain Go calling `docker`/`git`,
// so it runs identically on macOS, Linux, and Windows with no shell dependency.
package care

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Engine runs CARE actions against a kit directory (the folder holding
// docker-compose.yml, the env files, and the mounted configs).
type Engine struct {
	Kit string            // dir with docker-compose.yml, *.env, clinic_settings.py, ...
	Env map[string]string // overrides: BACKUP_DIR, CARE_MDNS_NAME, CARE_ADMIN_PASSWORD, CARE_NO_MDNS
	Log func(string)      // optional sink for streamed output (one line at a time)
	// Confirm asks the user a yes/no question (native dialog in the app). When
	// nil, callers treat it as "no" - never block a headless/CLI run.
	Confirm func(title, message string) bool

	versions map[string]string // parsed versions.env (lazy)
	once     sync.Once
}

func (e *Engine) logln(s string) {
	if e.Log != nil {
		e.Log(s)
	}
}

// get resolves a setting: explicit Env override > process env > versions.env > default.
func (e *Engine) get(key, def string) string {
	if e.Env != nil {
		if v, ok := e.Env[key]; ok && v != "" {
			return v
		}
	}
	if v := os.Getenv(key); v != "" {
		return v
	}
	e.once.Do(e.loadVersions)
	if v, ok := e.versions[key]; ok && v != "" {
		return v
	}
	return def
}

func (e *Engine) loadVersions() {
	e.versions = map[string]string{}
	b, err := os.ReadFile(filepath.Join(e.Kit, "versions.env"))
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			e.versions[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
}

// --- settings (mirror care.sh defaults) -------------------------------------

func (e *Engine) backendImage() string  { return e.get("BACKEND_IMAGE", "care:clinic") }
func (e *Engine) frontendImage() string { return e.get("FRONTEND_IMAGE", "care_fe:clinic") }
func (e *Engine) postgresImage() string { return e.get("POSTGRES_IMAGE", "postgres:17.10-alpine") }
func (e *Engine) redisImage() string    { return e.get("REDIS_IMAGE", "redis:8.8.0-alpine") }
func (e *Engine) minioImage() string {
	return e.get("MINIO_IMAGE", "minio/minio:RELEASE.2025-09-07T16-13-09Z")
}
func (e *Engine) caddyImage() string  { return e.get("CADDY_IMAGE", "caddy:2.11.4") }
func (e *Engine) backupImage() string { return e.get("BACKUP_IMAGE", "care-backup:clinic") }

// wafCaddyImage is the custom Caddy image (Coraza WAF compiled in) the caddy service
// actually runs; caddyImage() above is the pinned base it's built from.
func (e *Engine) wafCaddyImage() string { return e.get("CADDY_WAF_IMAGE", "care-caddy:clinic") }

// backupPassword is the passphrase that protects the backup keypair's private key.
// Empty means backup encryption is disabled - dumps are written in plaintext, and
// setup skips keypair generation. Set via CARE_BACKUP_PASSWORD at setup time.
func (e *Engine) backupPassword() string { return e.get("CARE_BACKUP_PASSWORD", "") }
func (e *Engine) beRepo() string {
	return e.get("CARE_BE_REPO", "https://github.com/ohcnetwork/care.git")
}
func (e *Engine) feRepo() string {
	return e.get("CARE_FE_REPO", "https://github.com/ohcnetwork/care_fe.git")
}
func (e *Engine) beRef() string         { return e.get("CARE_BE_REF", "develop") }
func (e *Engine) feRef() string         { return e.get("CARE_FE_REF", "develop") }
func (e *Engine) beDir() string         { return e.get("CARE_BE_DIR", filepath.Join(e.Kit, "care")) }
func (e *Engine) feDir() string         { return e.get("CARE_FE_DIR", filepath.Join(e.Kit, "care_fe")) }
func (e *Engine) mdnsName() string      { return e.get("CARE_MDNS_NAME", "care") }
func (e *Engine) adminPassword() string { return e.get("CARE_ADMIN_PASSWORD", "admin") }
func (e *Engine) noMDNS() bool          { return e.get("CARE_NO_MDNS", "0") == "1" }

// MDNSName is the bare host label to advertise/resolve (e.g. "care"). Exported for
// the app + CLI, which host the responder.
func (e *Engine) MDNSName() string { return e.mdnsName() }

// MDNSMode selects how http://<name>.local is made resolvable:
//   - "advertise" (default): a pure-Go mDNS responder in the app / `care mdns` -
//     no rename, no sudo, works on all 3 OSes.
//   - "rename": the old scutil/hostnamectl path (a permanent OS-level hostname).
//   - "off": do nothing (static-IP users). CARE_NO_MDNS=1 forces this too.
func (e *Engine) MDNSMode() string {
	if e.noMDNS() {
		return "off"
	}
	switch strings.ToLower(e.get("CARE_MDNS_MODE", "advertise")) {
	case "rename":
		return "rename"
	case "off":
		return "off"
	default:
		return "advertise"
	}
}

func (e *Engine) backupDir() string {
	if d := e.get("BACKUP_DIR", ""); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Desktop", "care-db-backups")
}

// --- process plumbing -------------------------------------------------------

// augmentedPath prepends the dirs where docker/git live (a GUI-launched app
// inherits a minimal PATH), with the right separator per OS.
func augmentedPath() string {
	var parts []string
	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
		parts = []string{
			`C:\Program Files\Docker\Docker\resources\bin`,
			`C:\Program Files\Git\bin`,
			`C:\Program Files\Git\cmd`,
		}
	} else {
		// include /usr/sbin + /sbin (scutil, hostname) and homebrew sbin - a
		// Finder-launched .app gets a minimal PATH that often omits these.
		parts = []string{
			"/opt/homebrew/bin", "/opt/homebrew/sbin",
			"/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin",
		}
	}
	if existing := os.Getenv("PATH"); existing != "" {
		parts = append(parts, existing)
	}
	return strings.Join(parts, sep)
}

// FixPath augments the *process* PATH so binary lookups succeed. exec.Command
// resolves a program against the process PATH (os.Getenv), not a command's Env -
// so a GUI-launched macOS app (minimal launchd PATH: /usr/bin:/bin:/usr/sbin:/sbin)
// can't find docker/git until we widen it. Call once at startup.
//
// Order of preference, most authoritative first:
//  1. the user's login-shell PATH (unix) - reflects wherever docker was actually
//     installed, since installers update the shell PATH (homebrew, colima, ~/.docker/bin...);
//  2. the current process PATH (on Windows this already has everything);
//  3. a few common dirs as a last-resort fallback.
func FixPath() {
	var parts []string
	if sp := loginShellPath(); sp != "" {
		parts = append(parts, sp)
	}
	parts = append(parts, augmentedPath()) // current PATH + common fallback dirs
	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
	}
	_ = os.Setenv("PATH", strings.Join(parts, sep))
}

// loginShellPath asks the user's login shell for its PATH - the same PATH the
// terminal sees, where docker/git are known to work. Unix only; bounded by a
// timeout so a slow/broken shell profile can't hang startup.
func loginShellPath() string {
	if runtime.GOOS == "windows" {
		return ""
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, shell, "-lc", "echo $PATH").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// baseEnv is the environment every docker/git call gets: the inherited env, an
// augmented PATH, and the vars docker-compose.yml reads.
func (e *Engine) baseEnv() []string {
	env := os.Environ()
	set := func(k, v string) { env = append(env, k+"="+v) }
	set("PATH", augmentedPath())
	set("BACKEND_IMAGE", e.backendImage())
	set("FRONTEND_IMAGE", e.frontendImage())
	set("POSTGRES_IMAGE", e.postgresImage())
	set("REDIS_IMAGE", e.redisImage())
	set("MINIO_IMAGE", e.minioImage())
	set("CADDY_IMAGE", e.caddyImage())
	set("CADDY_WAF_IMAGE", e.wafCaddyImage())
	set("BACKUP_IMAGE", e.backupImage())
	set("MINIO_ACCESS_KEY", "minioadmin")
	set("MINIO_SECRET_KEY", "minioadmin")
	set("BACKUP_DIR", e.backupDir())
	for k, v := range e.Env {
		set(k, v)
	}
	return env
}

// workdir returns the kit dir only if it exists - before setup it doesn't, and a
// command with a missing Dir fails to start (which silently broke the pre-setup
// scutil/hostname checks). Empty means "inherit the current dir".
func (e *Engine) workdir() string {
	if st, err := os.Stat(e.Kit); err == nil && st.IsDir() {
		return e.Kit
	}
	return ""
}

// newCmd builds an exec.Cmd with the console window hidden on Windows - a GUI
// app that polls docker must not flash a cmd window on every call. No-op on
// other platforms. All command spawns in this package go through here.
func newCmd(name string, args ...string) *exec.Cmd {
	c := exec.Command(name, args...)
	hideConsole(c)
	return c
}

// run executes a command in the kit dir and streams stdout+stderr to Log.
func (e *Engine) run(extraEnv []string, name string, args ...string) error {
	cmd := newCmd(name, args...)
	cmd.Dir = e.workdir()
	cmd.Env = append(e.baseEnv(), extraEnv...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", name, err)
	}
	var wg sync.WaitGroup
	stream := func(r io.Reader) {
		defer wg.Done()
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			e.logln(sc.Text())
		}
	}
	wg.Add(2)
	go stream(stdout)
	go stream(stderr)
	wg.Wait()
	return cmd.Wait()
}

// capture runs a command and returns its trimmed stdout (no streaming).
func (e *Engine) capture(name string, args ...string) (string, error) {
	cmd := newCmd(name, args...)
	cmd.Dir = e.workdir()
	cmd.Env = e.baseEnv()
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// dc runs `docker compose <args>` (streamed). Project name comes from the
// compose `name:` key - we never pass -v, so volumes/data always survive.
func (e *Engine) dc(args ...string) error {
	return e.run(nil, "docker", append([]string{"compose"}, args...)...)
}
