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
	Env map[string]string // overrides: BACKUP_DIR, CARE_ADMIN_PASSWORD, CARE_PUBLIC_HOST, ...
	Log func(string)      // optional sink for streamed output (one line at a time)

	settings map[string]string // parsed versions.env + tls.env (lazy)
	once     sync.Once
}

func (e *Engine) logln(s string) {
	if e.Log != nil {
		e.Log(s)
	}
}

// get resolves a setting: explicit Env override > process env > kit env files > default.
func (e *Engine) get(key, def string) string {
	if e.Env != nil {
		if v, ok := e.Env[key]; ok && v != "" {
			return v
		}
	}
	if v := os.Getenv(key); v != "" {
		return v
	}
	e.once.Do(e.loadSettings)
	if v, ok := e.settings[key]; ok && v != "" {
		return v
	}
	return def
}

// loadSettings reads the kit's plain key=value files. Both are optional - a missing
// one just means "no overrides from there".
func (e *Engine) loadSettings() {
	e.settings = map[string]string{}
	for _, name := range []string{"versions.env", "tls.env"} {
		b, err := os.ReadFile(filepath.Join(e.Kit, name))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if k, v, ok := strings.Cut(line, "="); ok {
				e.settings[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
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

// wafCaddyImage is the custom Caddy image (Coraza WAF + Cloudflare DNS compiled in)
// the caddy service actually runs; caddyImage() above is the pinned base it's built
// from. The tag carries "-tls" because ensureCaddyImage only builds when the image is
// *missing* - without a new tag, an upgraded install would keep running the old
// binary, which has no DNS module and fails at certificate issuance.
func (e *Engine) wafCaddyImage() string { return e.get("CADDY_WAF_IMAGE", "care-caddy:clinic-tls") }

// --- HTTPS (see tls.env + docs/tls.md) ---------------------------------------
//
// CARE is served over HTTPS on the clinic's own domain, and only that. There is no
// plain-HTTP mode: on a shared network a plaintext door can be intercepted and
// stripped, which would undo the encryption it hands off to. So publicHost is
// required, not optional - ValidateTLS refuses to start without it.

// acmeProduction is Let's Encrypt's live directory. Override with CARE_ACME_CA
// (staging) while testing - production rate-limits duplicate certs to 5 per week.
const acmeProduction = "https://acme-v02.api.letsencrypt.org/directory"

// httpsPort is the only port published to the clinic network.
const httpsPort = 443

// publicHost is the clinic's public DNS name (e.g. clinic.example.com). Everything
// else derives from it, so there's no second setting to fall out of sync.
func (e *Engine) publicHost() string { return strings.TrimSpace(e.get("CARE_PUBLIC_HOST", "")) }
func (e *Engine) dnsToken() string   { return e.get("CLOUDFLARE_API_TOKEN", "") }
func (e *Engine) acmeCA() string     { return e.get("CARE_ACME_CA", acmeProduction) }

// hstsSeconds is how long browsers remember to use HTTPS only for this clinic.
// Default 30 days - long enough that daily staff devices stay continuously protected,
// short enough that a mistake ages out in a month rather than a year.
func (e *Engine) hstsSeconds() string { return e.get("CARE_HSTS_SECONDS", "2592000") }

// Configured reports whether this install has a public name yet - false on a fresh
// box before the wizard has run.
func (e *Engine) Configured() bool { return e.publicHost() != "" }

// PublicOrigin is the scheme+host every device uses - what the frontend is built
// against, what Django signs file URLs with, and what CSRF trusts. Empty until
// configured. Exported because the app and CLI both surface it to the user.
func (e *Engine) PublicOrigin() string {
	if h := e.publicHost(); h != "" {
		return "https://" + h
	}
	return ""
}

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
func (e *Engine) adminPassword() string { return e.get("CARE_ADMIN_PASSWORD", "admin") }

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
	// HTTPS: the name Caddy serves and gets a certificate for, and the origin the
	// backend signs file URLs and trusts CSRF against.
	set("CARE_PUBLIC_HOST", e.publicHost())
	set("CARE_PUBLIC_ORIGIN", e.PublicOrigin())
	set("CLOUDFLARE_API_TOKEN", e.dnsToken())
	set("CARE_ACME_CA", e.acmeCA())
	set("CARE_HSTS_SECONDS", e.hstsSeconds())
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

// run executes a command in the kit dir and streams stdout+stderr to Log.
func (e *Engine) run(extraEnv []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
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
	cmd := exec.Command(name, args...)
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
