// Package clinic is the action layer: one exported method per thing an operator
// can do to the CARE stack. Every action is plain Go calling `docker`/`git`, so
// it runs identically on macOS, Linux, and Windows with no shell dependency.
// See docs/behaviour-contract.md.
package clinic

import (
	"os"
	"os/exec"
	"sync"

	"github.com/ohcnetwork/care_desktop/app/internal/compose"
	"github.com/ohcnetwork/care_desktop/app/internal/settings"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

// Clinic runs CARE actions against an install directory (the folder holding
// docker-compose.yml, the env files, and the mounted configs).
type Clinic struct {
	InstallDir string            // dir with docker-compose.yml, *.env, clinic_settings.py, ...
	Env        map[string]string // overrides: BACKUP_DIR, CARE_MDNS_NAME, CARE_ADMIN_PASSWORD, CARE_NO_MDNS
	Log        func(string)      // optional sink for streamed output (one line at a time)
	// Confirm asks the user a yes/no question (native dialog in the app). When
	// nil, callers treat it as "no" - never block a headless/CLI run.
	Confirm func(title, message string) bool

	set  *settings.Settings
	once sync.Once
}

// cfg lazily binds the settings reader to this engine's dir and overrides.
func (e *Clinic) cfg() *settings.Settings {
	e.once.Do(func() { e.set = &settings.Settings{Dir: e.InstallDir, Env: e.Env} })
	return e.set
}

func (e *Clinic) logln(s string) {
	if e.Log != nil {
		e.Log(s)
	}
}

// --- process plumbing -------------------------------------------------------

// baseEnv is the environment every docker/git call gets: the inherited env, an
// augmented PATH, and the vars docker-compose.yml reads.
func (e *Clinic) baseEnv() []string {
	env := os.Environ()
	set := func(k, v string) { env = append(env, k+"="+v) }
	set("PATH", proc.AugmentedPath())
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

// workdir returns the install dir only if it exists - before setup it doesn't, and a
// command with a missing Dir fails to start (which silently broke the pre-setup
// scutil/hostname checks). Empty means "inherit the current dir".
func (e *Clinic) workdir() string {
	if st, err := os.Stat(e.InstallDir); err == nil && st.IsDir() {
		return e.InstallDir
	}
	return ""
}

func newCmd(name string, args ...string) *exec.Cmd { return proc.Command(name, args...) }

// Runner exposes the engine's configured command runner.
func (e *Clinic) Runner() proc.Runner {
	return proc.Runner{Dir: e.workdir(), Env: e.baseEnv(), Log: e.Log}
}

func (e *Clinic) run(extraEnv []string, name string, args ...string) error {
	return e.Runner().RunWith(extraEnv, name, args...)
}

func (e *Clinic) capture(name string, args ...string) (string, error) {
	return e.Runner().Capture(name, args...)
}

func (e *Clinic) captureLines(name string, args ...string) []string {
	return e.Runner().Lines(name, args...)
}

// dc runs `docker compose <args>` (streamed). Project name comes from the
// compose `name:` key - we never pass -v, so volumes/data always survive.
func (e *Clinic) dc(args ...string) error {
	return e.run(nil, "docker", append([]string{"compose"}, args...)...)
}

// Settings forwarders. See internal/settings.
func (e *Clinic) backendImage() string   { return e.cfg().BackendImage() }
func (e *Clinic) frontendImage() string  { return e.cfg().FrontendImage() }
func (e *Clinic) postgresImage() string  { return e.cfg().PostgresImage() }
func (e *Clinic) redisImage() string     { return e.cfg().RedisImage() }
func (e *Clinic) minioImage() string     { return e.cfg().MinioImage() }
func (e *Clinic) caddyImage() string     { return e.cfg().CaddyImage() }
func (e *Clinic) backupImage() string    { return e.cfg().BackupImage() }
func (e *Clinic) wafCaddyImage() string  { return e.cfg().WafCaddyImage() }
func (e *Clinic) backupPassword() string { return e.cfg().BackupPassword() }
func (e *Clinic) beDir() string          { return e.cfg().BeDir() }
func (e *Clinic) feDir() string          { return e.cfg().FeDir() }
func (e *Clinic) mdnsName() string       { return e.cfg().MDNSName() }
func (e *Clinic) adminPassword() string  { return e.cfg().AdminPassword() }
func (e *Clinic) backupDir() string      { return e.cfg().BackupDir() }

// MDNSName is the bare host label to advertise/resolve (e.g. "care").
func (e *Clinic) MDNSName() string { return e.cfg().MDNSName() }

// MDNSMode selects how http://<name>.local is made resolvable.
func (e *Clinic) MDNSMode() string { return e.cfg().MDNSMode() }

// Builder binds an image builder to this engine's settings and runner.
func (e *Clinic) Builder() *compose.Builder {
	return compose.NewBuilder(e.Runner(), e.cfg(), e.Log)
}
