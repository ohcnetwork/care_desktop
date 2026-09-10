// Package compose builds the container images CARE runs, downloading the source
// repositories they are built from. See docs/architecture.md.
package compose

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ohcnetwork/care_desktop/app/internal/plugins"
	"github.com/ohcnetwork/care_desktop/app/internal/release"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

// Builder builds and inspects images for one install directory.
type Builder struct {
	Log func(string)

	dir string // install dir: the build context for the backup and Caddy images
	set *release.Pins
	run proc.Runner
}

// NewBuilder binds a Builder to an install dir, its release pins, and a runner.
func NewBuilder(run proc.Runner, dir string, set *release.Pins, log func(string)) *Builder {
	return &Builder{Log: log, dir: dir, set: set, run: run}
}

func (b *Builder) logln(s string) {
	if b.Log != nil {
		b.Log(s)
	}
}

// clone makes a shallow checkout of ref in a fresh temporary directory and returns
// it with a cleanup func. Nothing is cached between builds on purpose: a kept clone
// is only reused when it already exists, which silently pins the source to whatever
// ref was current the first time and ignores every later change to CARE_BE_REF.
func (b *Builder) clone(repo, ref, label string) (dir string, cleanup func(), err error) {
	dir, err = os.MkdirTemp("", "care-"+label+"-")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	b.logln("Downloading the CARE " + label + " source (" + ref + ")...")
	if err := b.run.Run("git", "clone", "--depth", "1", "--branch", ref, repo, dir); err != nil {
		cleanup()
		return "", nil, err
	}
	return dir, cleanup, nil
}

func (b *Builder) BuildBackend() error {
	src, cleanup, err := b.clone(b.set.BeRepo, b.set.BeRef, "backend")
	if err != nil {
		return err
	}
	defer cleanup()
	b.logln("Building the backend image (" + b.set.BackendImage + ")... (several minutes)")
	df := filepath.Join(src, "docker", "prod.Dockerfile")
	args := []string{"build", "-f", df, "-t", b.set.BackendImage, "--label", builtFromLabel + "=" + b.set.BeRef}
	// CARE pip-installs plugins at build time from ADDITIONAL_PLUGS.
	if plugs := plugins.New(b.run, b.dir, b.Log).AdditionalPlugs(); plugs != "" {
		b.logln("Building with plugins (ADDITIONAL_PLUGS set)")
		args = append(args, "--build-arg", "ADDITIONAL_PLUGS="+plugs)
	}
	args = append(args, src)
	return b.run.Run("docker", args...)
}

func (b *Builder) EnsureBackendImage() error {
	return b.ensure(b.set.BackendImage, b.set.BeRef, "backend", b.BuildBackend)
}

// BuildBackup builds the Postgres+openssl backup image.
func (b *Builder) BuildBackup() error {
	b.logln("Building the backup image (" + b.set.BackupImage + ")...")
	df := filepath.Join(b.dir, "backup.Dockerfile")
	return b.run.Run("docker", "build", "-f", df,
		"--build-arg", "POSTGRES_IMAGE="+b.set.PostgresImage,
		"--label", builtFromLabel+"="+b.set.PostgresImage,
		"-t", b.set.BackupImage, b.dir)
}

func (b *Builder) EnsureBackupImage() error {
	return b.ensure(b.set.BackupImage, b.set.PostgresImage, "backup", b.BuildBackup)
}

// BuildCaddy builds the reverse-proxy image with the Coraza WAF compiled in (xcaddy).
func (b *Builder) BuildCaddy() error {
	b.logln("Building the Caddy + WAF image (" + b.set.CaddyWafImage + ")... (compiles Caddy; a few minutes)")
	df := filepath.Join(b.dir, "caddy.Dockerfile")
	return b.run.Run("docker", "build", "-f", df,
		"--build-arg", "CADDY_IMAGE="+b.set.CaddyImage,
		"--label", builtFromLabel+"="+b.set.CaddyImage,
		"-t", b.set.CaddyWafImage, b.dir)
}

func (b *Builder) EnsureCaddyImage() error {
	return b.ensure(b.set.CaddyWafImage, b.set.CaddyImage, "caddy", b.BuildCaddy)
}

func (b *Builder) BuildFrontend() error {
	src, cleanup, err := b.clone(b.set.FeRepo, b.set.FeRef, "frontend")
	if err != nil {
		return err
	}
	defer cleanup()
	// frontend.env overrides care_fe's committed .env (Vite reads .env.local).
	env, err := os.ReadFile(filepath.Join(b.dir, "frontend.env"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(src, ".env.local"), env, 0o644); err != nil {
		return err
	}
	b.logln("Building the frontend image (" + b.set.FrontendImage + ")... (a few minutes)")
	return b.run.Run("docker", "build", "-t", b.set.FrontendImage,
		"--label", builtFromLabel+"="+b.set.FeRef, src)
}

func (b *Builder) EnsureFrontendImage() error {
	return b.ensure(b.set.FrontendImage, b.set.FeRef, "frontend", b.BuildFrontend)
}

// builtFromLabel records the pin an image was built from, so a changed pin can be
// detected. Tags are fixed (care-backup:clinic and friends), so tag existence alone
// cannot tell a stale image from a current one: after a POSTGRES_IMAGE bump the old
// backup image still exists, and its older pg_dump then refuses to dump the newer
// server, silently ending backups.
const builtFromLabel = "org.opencontainers.image.base.name"

// ensure builds tag unless it already exists AND was built from want. An image with
// no label predates this check and is treated as stale: one rebuild is cheaper than
// running an image we cannot account for.
func (b *Builder) ensure(tag, want, label string, build func() error) error {
	got, ok := b.builtFrom(tag)
	switch {
	case !ok:
		return build()
	case got == want:
		return nil
	}
	b.logln("The " + label + " image was built from " + quoteOrUnknown(got) +
		", but this release expects " + want + " - rebuilding.")
	return build()
}

// builtFrom reports the recorded input of an existing image. ok is false when the
// image is absent.
func (b *Builder) builtFrom(tag string) (value string, ok bool) {
	cmd := proc.Command("docker", "image", "inspect", "-f",
		"{{index .Config.Labels \""+builtFromLabel+"\"}}", tag)
	cmd.Env = b.run.Env
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	v := strings.TrimSpace(string(out))
	if v == "<no value>" {
		v = ""
	}
	return v, true
}

func quoteOrUnknown(s string) string {
	if s == "" {
		return "an unrecorded version"
	}
	return s
}
