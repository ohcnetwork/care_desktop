// Package clinic is the action layer: one exported method per thing an operator
// can do to the CARE stack. Every action is plain Go calling `docker`/`git`, so
// it runs identically on macOS, Linux, and Windows with no shell dependency.
// See docs/behaviour-contract.md.
package clinic

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/compose-spec/compose-go/v2/dotenv"
	"github.com/ohcnetwork/care_desktop/app/internal/compose"
	"github.com/ohcnetwork/care_desktop/app/internal/release"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

// Clinic runs CARE actions against an install directory (the folder holding
// docker-compose.yml, the env files, and the mounted configs).
type Clinic struct {
	InstallDir string // dir with docker-compose.yml, *.env, clinic_settings.py, ...

	// Operator choices, supplied by the caller for the run that needs them. They
	// are deliberately not read from the environment or .env: Compose interpolates
	// .env into every service, so a password there would be visible stack-wide.
	MDNSName       string // clinic address label, without ".local" (default "care")
	AdminPassword  string // CARE superuser password; empty means don't create one
	BackupPassword string // encrypts backups; required - there is no plaintext path
	BackupDir      string // where backups are written (default ~/Desktop/care-db-backups)

	// Pins are the release pins (images, source refs), loaded and validated once
	// at startup from the embedded .env. Every action needs them.
	Pins *release.Pins

	Log func(string) // optional sink for streamed output (one line at a time)
	// Confirm asks the user a yes/no question (native dialog in the app). When
	// nil, callers treat it as "no" - never block on a missing UI.
	Confirm func(title, message string) bool
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
	set("BACKEND_IMAGE", e.Pins.BackendImage)
	set("FRONTEND_IMAGE", e.Pins.FrontendImage)
	set("POSTGRES_IMAGE", e.Pins.PostgresImage)
	set("REDIS_IMAGE", e.Pins.RedisImage)
	set("MINIO_IMAGE", e.Pins.MinioImage)
	set("CADDY_IMAGE", e.Pins.CaddyImage)
	set("CADDY_WAF_IMAGE", e.Pins.CaddyWafImage)
	set("BACKUP_IMAGE", e.Pins.BackupImage)
	// MinIO's root credentials must match what the backend authenticates with.
	// They are read from backend.env - the file the UI edits - because hardcoding
	// them here meant changing them in Settings moved the backend's key while
	// MinIO kept the old root user, and uploads failed with an auth error that
	// pointed nowhere near the cause.
	creds := e.minioCreds()
	set("MINIO_ACCESS_KEY", creds[0])
	set("MINIO_SECRET_KEY", creds[1])
	set("BACKUP_DIR", e.backupDir())
	return env
}

// minioCreds returns MinIO's access key and secret from backend.env, falling
// back to MinIO's own defaults only when the file cannot be read (before setup,
// when no container will be started anyway).
func (e *Clinic) minioCreds() [2]string {
	out := [2]string{"minioadmin", "minioadmin"}
	b, err := os.ReadFile(filepath.Join(e.InstallDir, "backend.env"))
	if err != nil {
		return out
	}
	env, err := dotenv.Parse(bytes.NewReader(b))
	if err != nil {
		return out
	}
	if v := env["MINIO_ACCESS_KEY"]; v != "" {
		out[0] = v
	}
	if v := env["MINIO_SECRET_KEY"]; v != "" {
		out[1] = v
	}
	return out
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

func (e *Clinic) mdnsName() string {
	if e.MDNSName != "" {
		return e.MDNSName
	}
	return "care"
}

func (e *Clinic) backupDir() string {
	if e.BackupDir != "" {
		return e.BackupDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Desktop", "care-db-backups")
}

// Label is the bare host label to advertise, without ".local" (e.g. "care").
func (e *Clinic) Label() string { return e.mdnsName() }

// Builder binds an image builder to this engine's settings and runner.
func (e *Clinic) Builder() *compose.Builder {
	return compose.NewBuilder(e.Runner(), e.InstallDir, e.Pins, e.Log)
}
