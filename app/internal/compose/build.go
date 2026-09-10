// Package compose builds the container images CARE runs and clones the source
// repositories they are built from. See docs/architecture.md.
package compose

import (
	"os"
	"path/filepath"

	"github.com/ohcnetwork/care_desktop/app/internal/plugins"
	"github.com/ohcnetwork/care_desktop/app/internal/settings"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

// Builder builds and inspects images for one install directory.
type Builder struct {
	Log func(string)

	set *settings.Settings
	run proc.Runner
}

// NewBuilder binds a Builder to a settings reader and a command runner.
func NewBuilder(run proc.Runner, set *settings.Settings, log func(string)) *Builder {
	return &Builder{Log: log, set: set, run: run}
}

func (b *Builder) logln(s string) {
	if b.Log != nil {
		b.Log(s)
	}
}

func (b *Builder) Clone(repo, ref, dir, label string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return nil // already cloned
	}
	b.logln("Cloning care " + label + " (" + ref + ") -> " + dir)
	return b.run.Run("git", "Clone", "--depth", "1", "--branch", ref, repo, dir)
}

func (b *Builder) BuildBackend() error {
	if err := b.Clone(b.set.BeRepo(), b.set.BeRef(), b.set.BeDir(), "backend"); err != nil {
		return err
	}
	b.logln("Building the backend image (" + b.set.BackendImage() + ")... (several minutes)")
	df := filepath.Join(b.set.BeDir(), "docker", "prod.Dockerfile")
	args := []string{"build", "-f", df, "-t", b.set.BackendImage()}
	// CARE pip-installs plugins at build time from ADDITIONAL_PLUGS.
	if plugs := plugins.New(b.run, b.set.Dir, b.Log).AdditionalPlugs(); plugs != "" {
		b.logln("Building with plugins (ADDITIONAL_PLUGS set)")
		args = append(args, "--build-arg", "ADDITIONAL_PLUGS="+plugs)
	}
	args = append(args, b.set.BeDir())
	return b.run.Run("docker", args...)
}

func (b *Builder) EnsureBackendImage() error {
	if b.imageExists(b.set.BackendImage()) {
		return nil
	}
	return b.BuildBackend()
}

// BuildBackup builds the Postgres+openssl backup image. Runs before the clones land
// so the daemon isn't sent the (unused) hundreds of MB of source as build context.
func (b *Builder) BuildBackup() error {
	b.logln("Building the backup image (" + b.set.BackupImage() + ")...")
	df := filepath.Join(b.set.Dir, "backup.Dockerfile")
	return b.run.Run("docker", "build", "-f", df,
		"--build-arg", "POSTGRES_IMAGE="+b.set.PostgresImage(),
		"-t", b.set.BackupImage(), b.set.Dir)
}

func (b *Builder) EnsureBackupImage() error {
	if b.imageExists(b.set.BackupImage()) {
		return nil
	}
	return b.BuildBackup()
}

// BuildCaddy builds the reverse-proxy image with the Coraza WAF compiled in (xcaddy).
func (b *Builder) BuildCaddy() error {
	b.logln("Building the Caddy + WAF image (" + b.set.WafCaddyImage() + ")... (compiles Caddy; a few minutes)")
	df := filepath.Join(b.set.Dir, "caddy.Dockerfile")
	return b.run.Run("docker", "build", "-f", df,
		"--build-arg", "CADDY_IMAGE="+b.set.CaddyImage(),
		"-t", b.set.WafCaddyImage(), b.set.Dir)
}

func (b *Builder) EnsureCaddyImage() error {
	if b.imageExists(b.set.WafCaddyImage()) {
		return nil
	}
	return b.BuildCaddy()
}

func (b *Builder) BuildFrontend() error {
	if err := b.Clone(b.set.FeRepo(), b.set.FeRef(), b.set.FeDir(), "frontend"); err != nil {
		return err
	}
	// frontend.env overrides care_fe's committed .env (Vite reads .env.local).
	src, err := os.ReadFile(filepath.Join(b.set.Dir, "frontend.env"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(b.set.FeDir(), ".env.local"), src, 0o644); err != nil {
		return err
	}
	b.logln("Building the frontend image (" + b.set.FrontendImage() + ")... (a few minutes)")
	return b.run.Run("docker", "build", "-t", b.set.FrontendImage(), b.set.FeDir())
}

func (b *Builder) EnsureFrontendImage() error {
	if b.imageExists(b.set.FrontendImage()) {
		return nil
	}
	return b.BuildFrontend()
}

func (b *Builder) imageExists(tag string) bool {
	cmd := proc.Command("docker", "image", "inspect", tag)
	cmd.Env = b.run.Env
	return cmd.Run() == nil
}
