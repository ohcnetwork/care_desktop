package compose

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

func (b *Builder) source(repo, ref, label string) (string, error) {
	dir := filepath.Join(b.dir, "src", label)
	stamp := filepath.Join(dir, ".care-source")
	want := b.sourceKey(repo, ref)

	if got, err := os.ReadFile(stamp); err == nil {
		if strings.TrimSpace(string(got)) == want {
			b.logln("Using the CARE " + label + " source already downloaded (" + ref + ").")
			return dir, nil
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", err
	}
	ready := false
	defer func() {
		if !ready {
			_ = os.RemoveAll(dir)
		}
	}()
	b.logln("Downloading the CARE " + label + " source (" + ref + ")...")
	if err := b.run.Run("git", "init", "--quiet", "--", dir); err != nil {
		return "", err
	}
	if err := b.run.Run("git", "-C", dir, "remote", "add", "--", "origin", repo); err != nil {
		return "", err
	}
	if err := b.run.Run("git", "-C", dir, "fetch", "--depth", "1", "--", "origin", ref); err != nil {
		return "", err
	}
	if err := b.run.Run("git", "-C", dir, "checkout", "--detach", "FETCH_HEAD"); err != nil {
		return "", err
	}
	if release.IsCommitRef(ref) {
		head, err := b.run.Capture("git", "-C", dir, "rev-parse", "HEAD")
		if err != nil {
			return "", err
		}
		if !strings.EqualFold(head, ref) {
			return "", fmt.Errorf("CARE %s checkout is %s, expected %s", label, head, ref)
		}
	}
	if err := os.WriteFile(stamp, []byte(want), 0o644); err != nil {
		return "", err
	}
	ready = true
	return dir, nil
}

func (b *Builder) sourceKey(repo, ref string) string {
	return b.set.AppVersion + "+" + ref + "+repo@" + shortHash([]byte(repo))
}

func (b *Builder) BuildBackend() error {
	plugs, err := plugins.New(b.dir).AdditionalPlugs()
	if err != nil {
		return err
	}
	src, err := b.source(b.set.BeRepo, b.set.BeRef, "backend")
	if err != nil {
		return err
	}
	b.logln("Building the backend image (" + b.set.BackendImage + ")... (several minutes)")
	df := filepath.Join(src, "docker", "prod.Dockerfile")
	args := []string{"build", "-f", df, "-t", b.set.BackendImage,
		"--label", builtFromLabel + "=" + b.backendBuiltFrom(plugs)}
	if plugs != "" {
		b.logln("Building with plugins (ADDITIONAL_PLUGS set)")
		args = append(args, "--build-arg", "ADDITIONAL_PLUGS="+plugs)
	}
	args = append(args, src)
	return b.run.Run("docker", args...)
}

func (b *Builder) EnsureBackendImage() error {
	plugs, err := plugins.New(b.dir).AdditionalPlugs()
	if err != nil {
		return err
	}
	return b.ensure(b.set.BackendImage, b.backendBuiltFrom(plugs), "backend", b.BuildBackend)
}

func (b *Builder) backendBuiltFrom(plugs string) string {
	out := b.sourceKey(b.set.BeRepo, b.set.BeRef)
	if plugs != "" {
		out += "+plugs@" + shortHash([]byte(plugs))
	}
	return out
}

func emptyBuildContext(parent string) (dir string, cleanup func(), err error) {
	dir, err = os.MkdirTemp(parent, ".care-buildctx-")
	if err != nil {
		return "", nil, err
	}
	return dir, func() { _ = os.RemoveAll(dir) }, nil
}

func (b *Builder) BuildBackup() error {
	key, err := b.backupBuiltFrom()
	if err != nil {
		return err
	}
	b.logln("Building the backup image (" + b.set.BackupImage + ")...")
	df := filepath.Join(b.dir, "backup.Dockerfile")
	buildCtx, cleanup, err := emptyBuildContext(b.dir)
	if err != nil {
		return err
	}
	defer cleanup()
	return b.run.Run("docker", "build", "-f", df,
		"--build-arg", "POSTGRES_IMAGE="+b.set.PostgresImage,
		"--label", builtFromLabel+"="+key,
		"-t", b.set.BackupImage, buildCtx)
}

func (b *Builder) EnsureBackupImage() error {
	key, err := b.backupBuiltFrom()
	if err != nil {
		return err
	}
	return b.ensure(b.set.BackupImage, key, "backup", b.BuildBackup)
}

func (b *Builder) backupBuiltFrom() (string, error) {
	df, err := os.ReadFile(filepath.Join(b.dir, "backup.Dockerfile"))
	if err != nil {
		return "", err
	}
	return b.set.PostgresImage + "+dockerfile@" + shortHash(df), nil
}

func (b *Builder) BuildCaddy() error {
	key, err := b.caddyBuiltFrom()
	if err != nil {
		return err
	}
	b.logln("Building the Caddy + WAF image (" + b.set.CaddyWafImage + ")... (compiles Caddy; a few minutes)")
	df := filepath.Join(b.dir, "caddy.Dockerfile")
	buildCtx, cleanup, err := emptyBuildContext(b.dir)
	if err != nil {
		return err
	}
	defer cleanup()
	return b.run.Run("docker", "build", "-f", df,
		"--build-arg", "CADDY_IMAGE="+b.set.CaddyImage,
		"--build-arg", "CORAZA_VERSION="+b.set.CorazaVersion,
		"--label", builtFromLabel+"="+key,
		"-t", b.set.CaddyWafImage, buildCtx)
}

func (b *Builder) caddyBuiltFrom() (string, error) {
	df, err := os.ReadFile(filepath.Join(b.dir, "caddy.Dockerfile"))
	if err != nil {
		return "", err
	}
	return b.set.CaddyImage + "+coraza@" + b.set.CorazaVersion + "+dockerfile@" + shortHash(df), nil
}

func (b *Builder) EnsureCaddyImage() error {
	key, err := b.caddyBuiltFrom()
	if err != nil {
		return err
	}
	return b.ensure(b.set.CaddyWafImage, key, "caddy", b.BuildCaddy)
}

func (b *Builder) BuildFrontend() error {
	env, err := os.ReadFile(filepath.Join(b.dir, "frontend.env"))
	if err != nil {
		return err
	}
	src, err := b.source(b.set.FeRepo, b.set.FeRef, "frontend")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(src, ".env.local"), env, 0o644); err != nil {
		return err
	}
	b.logln("Building the frontend image (" + b.set.FrontendImage + ")... (a few minutes)")
	return b.run.Run("docker", "build", "-t", b.set.FrontendImage,
		"--label", builtFromLabel+"="+b.frontendBuiltFrom(env), src)
}

func (b *Builder) frontendBuiltFrom(env []byte) string {
	return b.sourceKey(b.set.FeRepo, b.set.FeRef) + "+env@" + shortHash(env)
}

func (b *Builder) EnsureFrontendImage() error {
	env, err := os.ReadFile(filepath.Join(b.dir, "frontend.env"))
	if err != nil {
		return err
	}
	return b.ensure(b.set.FrontendImage, b.frontendBuiltFrom(env), "frontend", b.BuildFrontend)
}

func shortHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:12]
}

const builtFromLabel = "org.opencontainers.image.base.name"

func (b *Builder) ensure(tag, want, label string, build func() error) error {
	got, ok, err := b.builtFrom(tag)
	if err != nil {
		return err
	}
	switch {
	case !ok:
		return build()
	case got == want:
		return nil
	}
	b.logln("The " + label + " image was built from " + quoteOrUnknown(got) +
		", but this release expects " + want + " - rebuilding.")
	if err := build(); err != nil {
		return err
	}
	b.pruneDangling()
	return nil
}

func (b *Builder) pruneDangling() {
	cmd := proc.Command("docker", "image", "prune", "-f")
	cmd.Env = b.run.Env
	if err := cmd.Run(); err == nil {
		b.logln("Cleared out images left behind by the rebuild.")
	}
}

func (b *Builder) builtFrom(tag string) (value string, ok bool, err error) {
	ids, err := b.run.Capture("docker", "image", "ls", "--quiet", "--filter", "reference="+tag)
	if err != nil {
		return "", false, fmt.Errorf("inspect local images: %w", err)
	}
	if ids == "" {
		return "", false, nil
	}
	out, err := b.run.Capture("docker", "image", "inspect", "-f",
		"{{index .Config.Labels \""+builtFromLabel+"\"}}", tag)
	if err != nil {
		return "", false, fmt.Errorf("inspect image %s: %w", tag, err)
	}
	v := strings.TrimSpace(out)
	if v == "<no value>" {
		v = ""
	}
	return v, true, nil
}

func quoteOrUnknown(s string) string {
	if s == "" {
		return "an unrecorded version"
	}
	return s
}
