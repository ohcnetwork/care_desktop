package compose

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ohcnetwork/care_desktop/app/internal/plugins"
	"github.com/ohcnetwork/care_desktop/app/internal/release"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

func writeBuildFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func builderFixture(t *testing.T) *Builder {
	t.Helper()
	dir := t.TempDir()
	for name, content := range map[string]string{
		"backend.env":       "ADDITIONAL_PLUGS='[{\"name\":\"one\",\"package_name\":\"pkg_one\"}]'\nPOSTGRES_HOST=db\n",
		"frontend.env":      "REACT_CARE_API_URL=https://care.local\n",
		"backup.Dockerfile": "ARG POSTGRES_IMAGE\nFROM ${POSTGRES_IMAGE}\nRUN apk add --no-cache openssl\n",
		"caddy.Dockerfile":  "ARG CADDY_IMAGE\nFROM ${CADDY_IMAGE}\nARG CORAZA_VERSION\n",
	} {
		writeBuildFile(t, dir, name, content)
	}
	return NewBuilder(proc.Runner{Dir: dir}, dir, &release.Pins{
		AppVersion:    "0.1.0-dev",
		BeRepo:        "https://github.com/ohcnetwork/care.git",
		FeRepo:        "https://github.com/ohcnetwork/care_fe.git",
		BeRef:         "develop",
		FeRef:         "develop",
		BackendImage:  "care:clinic",
		FrontendImage: "care_fe:clinic",
		BackupImage:   "care-backup:clinic",
		CaddyWafImage: "care-caddy:clinic",
		PostgresImage: "postgres:17-alpine",
		CaddyImage:    "caddy:2",
		CorazaVersion: "v2.6.1",
	}, nil)
}

func imageKeys(t *testing.T, b *Builder) [4]string {
	t.Helper()
	plugs, err := plugins.New(b.dir).AdditionalPlugs()
	if err != nil {
		t.Fatal(err)
	}
	env, err := os.ReadFile(filepath.Join(b.dir, "frontend.env"))
	if err != nil {
		t.Fatal(err)
	}
	backup, err := b.backupBuiltFrom()
	if err != nil {
		t.Fatal(err)
	}
	caddy, err := b.caddyBuiltFrom()
	if err != nil {
		t.Fatal(err)
	}
	return [4]string{b.backendBuiltFrom(plugs), b.frontendBuiltFrom(env), backup, caddy}
}

func TestImageKeysTrackBuildInputs(t *testing.T) {
	tests := []struct {
		name    string
		changed [4]bool
		mutate  func(*testing.T, *Builder)
	}{
		{"version", [4]bool{true, true}, func(t *testing.T, b *Builder) { b.set.AppVersion = "0.2.0-dev" }},
		{"backend repo", [4]bool{true}, func(t *testing.T, b *Builder) { b.set.BeRepo += "-fork" }},
		{"backend ref", [4]bool{true}, func(t *testing.T, b *Builder) { b.set.BeRef = "feature" }},
		{"frontend repo", [4]bool{false, true}, func(t *testing.T, b *Builder) { b.set.FeRepo += "-fork" }},
		{"frontend ref", [4]bool{false, true}, func(t *testing.T, b *Builder) { b.set.FeRef = "feature" }},
		{"plugins", [4]bool{true}, func(t *testing.T, b *Builder) {
			writeBuildFile(t, b.dir, "backend.env", "ADDITIONAL_PLUGS='[{\"name\":\"two\",\"package_name\":\"pkg_two\"}]'\n")
		}},
		{"frontend env", [4]bool{false, true}, func(t *testing.T, b *Builder) {
			writeBuildFile(t, b.dir, "frontend.env", "REACT_CARE_API_URL=https://other.local\n")
		}},
		{"postgres", [4]bool{false, false, true}, func(t *testing.T, b *Builder) { b.set.PostgresImage = "postgres:18-alpine" }},
		{"backup Dockerfile", [4]bool{false, false, true}, func(t *testing.T, b *Builder) {
			writeBuildFile(t, b.dir, "backup.Dockerfile", "ARG POSTGRES_IMAGE\nFROM ${POSTGRES_IMAGE}\nRUN apk add openssl tar\n")
		}},
		{"caddy", [4]bool{false, false, false, true}, func(t *testing.T, b *Builder) { b.set.CaddyImage = "caddy:2.11" }},
		{"coraza", [4]bool{false, false, false, true}, func(t *testing.T, b *Builder) { b.set.CorazaVersion = "v2.7.0" }},
		{"caddy Dockerfile", [4]bool{false, false, false, true}, func(t *testing.T, b *Builder) {
			writeBuildFile(t, b.dir, "caddy.Dockerfile", "ARG CADDY_IMAGE\nFROM ${CADDY_IMAGE}\nARG CORAZA_VERSION\nRUN caddy version\n")
		}},
		{"runtime files", [4]bool{}, func(t *testing.T, b *Builder) {
			writeBuildFile(t, b.dir, "backend.env", "POSTGRES_HOST=other-db\nBUCKET_KEY=custom-user\nFILE_UPLOAD_BUCKET=records\nADDITIONAL_PLUGS='[{\"name\":\"one\",\"package_name\":\"pkg_one\"}]'\n")
			writeBuildFile(t, b.dir, "Caddyfile", "other.local:443 {}\n")
			writeBuildFile(t, b.dir, "scripts/backup.sh", "exit 0\n")
			writeBuildFile(t, b.dir, "minio/entrypoint.sh", "exit 0\n")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := builderFixture(t)
			before := imageKeys(t, b)
			tt.mutate(t, b)
			after := imageKeys(t, b)
			for i, wantChanged := range tt.changed {
				if gotChanged := before[i] != after[i]; gotChanged != wantChanged {
					t.Errorf("image %d changed = %v, want %v", i, gotChanged, wantChanged)
				}
			}
		})
	}
}

func gitFixture(t *testing.T, repo string) proc.Runner {
	t.Helper()
	if !proc.Exists("git") {
		t.Skip("git is not installed")
	}
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	run := proc.Runner{Dir: repo}
	if err := run.Run("git", "init", "--quiet", "--initial-branch=develop"); err != nil {
		t.Fatal(err)
	}
	return run
}

func commitFixture(t *testing.T, run proc.Runner, content string) string {
	t.Helper()
	writeBuildFile(t, run.Dir, "revision", content)
	if err := run.Run("git", "add", "revision"); err != nil {
		t.Fatal(err)
	}
	if err := run.Run("git", "-c", "user.name=CARE tests", "-c", "user.email=care-tests@example.invalid",
		"commit", "--quiet", "--no-gpg-sign", "-m", content+"\n\nCo-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"); err != nil {
		t.Fatal(err)
	}
	head, err := run.Capture("git", "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return head
}

func TestSourceSupportsPinnedCommitsAndDevelopmentRefs(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "remote")
	run := gitFixture(t, repo)
	pinned := commitFixture(t, run, "pinned")
	moving := commitFixture(t, run, "moving")
	b := builderFixture(t)
	for _, tt := range []struct {
		ref   string
		label string
		head  string
		want  string
	}{
		{pinned, "backend", pinned, "pinned"},
		{"develop", "frontend", moving, "moving"},
	} {
		src, err := b.source(repo, tt.ref, tt.label)
		if err != nil {
			t.Fatal(err)
		}
		head, err := b.run.Capture("git", "-C", src, "rev-parse", "HEAD")
		if err != nil || head != tt.head {
			t.Fatalf("checkout HEAD = %q, want %q: %v", head, tt.head, err)
		}
		data, err := os.ReadFile(filepath.Join(src, "revision"))
		if err != nil || string(data) != tt.want {
			t.Fatalf("checkout content = %q, want %q: %v", data, tt.want, err)
		}
		stamp, err := os.ReadFile(filepath.Join(src, ".care-source"))
		if err != nil || string(stamp) != b.sourceKey(repo, tt.ref) {
			t.Fatalf("source stamp does not match its inputs: %q, %v", stamp, err)
		}
	}
	src, err := b.source(repo, moving, "backend")
	if err != nil {
		t.Fatal(err)
	}
	head, err := b.run.Capture("git", "-C", src, "rev-parse", "HEAD")
	if err != nil || head != moving {
		t.Fatalf("changed source ref reused the earlier commit: %q, %v", head, err)
	}
	if err := os.Rename(repo, filepath.Join(root, "offline")); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ ref, label string }{{moving, "backend"}, {"develop", "frontend"}} {
		if _, err := b.source(repo, tt.ref, tt.label); err != nil {
			t.Fatalf("cached source required network access: %v", err)
		}
	}
}

func TestSourceInvalidatesRepositoryAndLegacyStamps(t *testing.T) {
	root := t.TempDir()
	b := builderFixture(t)
	for _, name := range []string{"original", "fork"} {
		repo := filepath.Join(root, name)
		run := gitFixture(t, repo)
		commitFixture(t, run, name)
		src, err := b.source(repo, "develop", "backend")
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(src, "revision"))
		if err != nil || string(data) != name {
			t.Fatalf("source was reused from another repository: %q, %v", data, err)
		}
		writeBuildFile(t, src, ".care-source", b.set.AppVersion+"+develop")
		writeBuildFile(t, src, "revision", "stale")
		if _, err := b.source(repo, "develop", "backend"); err != nil {
			t.Fatal(err)
		}
		data, err = os.ReadFile(filepath.Join(src, "revision"))
		if err != nil || string(data) != name {
			t.Fatalf("legacy stamp was treated as fresh: %q, %v", data, err)
		}
	}
}

func TestSourceReadFailurePreservesCheckout(t *testing.T) {
	b := builderFixture(t)
	src := filepath.Join(b.dir, "src", "backend")
	writeBuildFile(t, src, ".care-source/keep", "keep")
	if _, err := b.source(b.set.BeRepo, b.set.BeRef, "backend"); err == nil {
		t.Fatal("an unreadable source stamp was ignored")
	}
	if _, err := os.Stat(filepath.Join(src, ".care-source", "keep")); err != nil {
		t.Fatal("checkout was deleted after a stamp read error")
	}
}

func TestFailedSourceFetchLeavesNoStamp(t *testing.T) {
	b := builderFixture(t)
	if !proc.Exists("git") {
		t.Skip("git is not installed")
	}
	if _, err := b.source(filepath.Join(b.dir, "absent-repository"), "develop", "backend"); err == nil {
		t.Fatal("missing source was accepted")
	}
	if _, err := os.Stat(filepath.Join(b.dir, "src", "backend")); !os.IsNotExist(err) {
		t.Fatalf("incomplete checkout was retained: %v", err)
	}
}

func dockerFixture(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX command fixture")
	}
	dir := t.TempDir()
	trace := filepath.Join(dir, "calls")
	t.Setenv("CARE_BUILD_TRACE", trace)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	script := `#!/bin/sh
printf '%s\n' "$@" >> "$CARE_BUILD_TRACE"
printf '%s\n' '--' >> "$CARE_BUILD_TRACE"
case "$1 $2" in
  'image ls')
    [ "$CARE_FAIL_LIST" != yes ] || exit 21
    [ "$CARE_IMAGE_EXISTS" != yes ] || printf 'image-id\n'
    ;;
  'image inspect')
    [ "$CARE_FAIL_INSPECT" != yes ] || exit 22
    printf '%s\n' "$CARE_IMAGE_KEY"
    ;;
  'image prune') ;;
  build*)
    for arg do context=$arg; done
    [ -d "$context" ] || exit 23
    ;;
  *) exit 99 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return trace
}

func TestEnsureDistinguishesMissingImagesFromInspectionFailures(t *testing.T) {
	for _, tt := range []struct {
		name    string
		exists  string
		key     string
		failure string
		builds  int
	}{
		{"missing", "", "", "", 1},
		{"current", "yes", "wanted", "", 0},
		{"stale", "yes", "older", "", 1},
		{"unrecorded", "yes", "<no value>", "", 1},
		{"list failure", "yes", "", "CARE_FAIL_LIST", 0},
		{"inspect failure", "yes", "", "CARE_FAIL_INSPECT", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dockerFixture(t)
			t.Setenv("CARE_IMAGE_EXISTS", tt.exists)
			t.Setenv("CARE_IMAGE_KEY", tt.key)
			if tt.failure != "" {
				t.Setenv(tt.failure, "yes")
			}
			builds := 0
			err := builderFixture(t).ensure("care:clinic", "wanted", "backend", func() error {
				builds++
				return nil
			})
			if (err != nil) != (tt.failure != "") {
				t.Errorf("unexpected inspection result: %v", err)
			}
			if builds != tt.builds {
				t.Errorf("built %d images, want %d", builds, tt.builds)
			}
		})
	}
}

func TestBuildAndEnsurePropagateInputReadErrors(t *testing.T) {
	for _, name := range []string{"backend.env", "frontend.env", "backup.Dockerfile", "caddy.Dockerfile"} {
		for _, failure := range []string{"missing", "directory"} {
			t.Run(name+"/"+failure, func(t *testing.T) {
				trace := dockerFixture(t)
				b := builderFixture(t)
				path := filepath.Join(b.dir, name)
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if failure == "directory" {
					if err := os.Mkdir(path, 0o700); err != nil {
						t.Fatal(err)
					}
				}
				operations := map[string][]func() error{
					"backend.env":       {b.BuildBackend, b.EnsureBackendImage},
					"frontend.env":      {b.BuildFrontend, b.EnsureFrontendImage},
					"backup.Dockerfile": {b.BuildBackup, b.EnsureBackupImage},
					"caddy.Dockerfile":  {b.BuildCaddy, b.EnsureCaddyImage},
				}
				for _, operation := range operations[name] {
					if err := operation(); err == nil {
						t.Fatal("unreadable build input was ignored")
					}
				}
				if _, err := os.Stat(trace); !os.IsNotExist(err) {
					t.Fatalf("Docker was called before inputs were readable: %v", err)
				}
			})
		}
	}
	t.Run("malformed plugins", func(t *testing.T) {
		trace := dockerFixture(t)
		b := builderFixture(t)
		writeBuildFile(t, b.dir, "backend.env", "ADDITIONAL_PLUGS='unterminated\n")
		for _, operation := range []func() error{b.BuildBackend, b.EnsureBackendImage} {
			if err := operation(); err == nil {
				t.Fatal("malformed build inputs were accepted")
			}
		}
		if _, err := os.Stat(trace); !os.IsNotExist(err) {
			t.Fatalf("Docker was called with malformed inputs: %v", err)
		}
	})
}

func TestBuildLabelsMatchConsumedInputs(t *testing.T) {
	trace := dockerFixture(t)
	b := builderFixture(t)
	for _, source := range []struct{ label, repo, ref string }{
		{"backend", b.set.BeRepo, b.set.BeRef},
		{"frontend", b.set.FeRepo, b.set.FeRef},
	} {
		src := filepath.Join(b.dir, "src", source.label)
		writeBuildFile(t, src, ".care-source", b.sourceKey(source.repo, source.ref))
		writeBuildFile(t, src, "Dockerfile", "FROM scratch\n")
		writeBuildFile(t, src, "docker/prod.Dockerfile", "FROM scratch\n")
	}
	keys := imageKeys(t, b)
	for _, build := range []func() error{b.BuildBackend, b.BuildFrontend, b.BuildBackup, b.BuildCaddy} {
		if err := build(); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range keys {
		if !strings.Contains(string(data), builtFromLabel+"="+key+"\n") {
			t.Errorf("build did not label its inputs: %s", key)
		}
	}
	plugs, err := plugins.New(b.dir).AdditionalPlugs()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "ADDITIONAL_PLUGS="+plugs+"\n") {
		t.Fatal("backend plugin input did not reach Docker")
	}
	env, err := os.ReadFile(filepath.Join(b.dir, "frontend.env"))
	if err != nil {
		t.Fatal(err)
	}
	baked, err := os.ReadFile(filepath.Join(b.dir, "src", "frontend", ".env.local"))
	if err != nil || string(baked) != string(env) {
		t.Fatalf("frontend build did not receive its hashed input: %v", err)
	}
	contexts, err := filepath.Glob(filepath.Join(b.dir, ".care-buildctx-*"))
	if err != nil || len(contexts) != 0 {
		t.Fatalf("empty build contexts were not cleaned up: %v, %v", contexts, err)
	}
}
