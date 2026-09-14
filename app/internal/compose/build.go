package compose

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ohcnetwork/care_desktop/app/internal/plugins"
	"github.com/ohcnetwork/care_desktop/app/internal/release"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

type Builder struct {
	Log func(string)

	dir string // install dir: the build context for the backup and Caddy images
	set *release.Pins
	run proc.Runner
}

func NewBuilder(run proc.Runner, dir string, set *release.Pins, log func(string)) *Builder {
	return &Builder{Log: log, dir: dir, set: set, run: run}
}

func (b *Builder) logln(s string) {
	if b.Log != nil {
		b.Log(s)
	}
}

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

func emptyBuildContext() (dir string, cleanup func(), err error) {
	dir, err = os.MkdirTemp("", "care-buildctx-")
	if err != nil {
		return "", nil, err
	}
	return dir, func() { _ = os.RemoveAll(dir) }, nil
}

func (b *Builder) BuildBackup() error {
	b.logln("Building the backup image (" + b.set.BackupImage + ")...")
	df := filepath.Join(b.dir, "backup.Dockerfile")
	buildCtx, cleanup, err := emptyBuildContext()
	if err != nil {
		return err
	}
	defer cleanup()
	return b.run.Run("docker", "build", "-f", df,
		"--build-arg", "POSTGRES_IMAGE="+b.set.PostgresImage,
		"--label", builtFromLabel+"="+b.set.PostgresImage,
		"-t", b.set.BackupImage, buildCtx)
}

func (b *Builder) EnsureBackupImage() error {
	return b.ensure(b.set.BackupImage, b.set.PostgresImage, "backup", b.BuildBackup)
}

func (b *Builder) BuildCaddy() error {
	b.logln("Building the Caddy + WAF image (" + b.set.CaddyWafImage + ")... (compiles Caddy; a few minutes)")
	df := filepath.Join(b.dir, "caddy.Dockerfile")
	buildCtx, cleanup, err := emptyBuildContext()
	if err != nil {
		return err
	}
	defer cleanup()
	return b.run.Run("docker", "build", "-f", df,
		"--build-arg", "CADDY_IMAGE="+b.set.CaddyImage,
		"--build-arg", "CORAZA_VERSION="+b.set.CorazaVersion,
		"--label", builtFromLabel+"="+b.caddyBuiltFrom(),
		"-t", b.set.CaddyWafImage, buildCtx)
}

func (b *Builder) caddyBuiltFrom() string {
	return b.set.CaddyImage + "+coraza@" + b.set.CorazaVersion
}

func (b *Builder) EnsureCaddyImage() error {
	return b.ensure(b.set.CaddyWafImage, b.caddyBuiltFrom(), "caddy", b.BuildCaddy)
}

func (b *Builder) BuildFrontend() error {
	src, cleanup, err := b.clone(b.set.FeRepo, b.set.FeRef, "frontend")
	if err != nil {
		return err
	}
	defer cleanup()
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

const builtFromLabel = "org.opencontainers.image.base.name"

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
