package residue

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"text/template"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

func TestUnavailableDockerIsNotClean(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX command fixtures")
	}
	root := t.TempDir()
	t.Setenv("HOME", root)
	for _, name := range []string{"docker", "security"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", root)
	run := proc.Runner{Env: os.Environ()}
	report, err := Scan(Options{
		Runner: run, Project: "care-desktop", InstallDir: filepath.Join(root, "install"),
	})
	if err == nil || report.Clean {
		t.Fatalf("unavailable Docker was called clean: %+v, %v", report, err)
	}
	if _, err := InstallDirFrom(run, "care-desktop", filepath.Join(root, "missing")); err == nil {
		t.Fatal("failed installation discovery was ignored")
	}
}

type dockerPsRow struct{ labels map[string]string }

func (r dockerPsRow) Labels() string {
	var out []string
	for k, v := range r.labels {
		out = append(out, k+"="+v)
	}
	return strings.Join(out, ",")
}

func (r dockerPsRow) Label(name string) string { return r.labels[name] }

func TestWorkingDirFormatMatchesDockerPsContext(t *testing.T) {
	tmpl, err := template.New("").Parse(workingDirFormat)
	if err != nil {
		t.Fatal(err)
	}
	row := dockerPsRow{labels: map[string]string{
		"com.docker.compose.project":             "care-desktop",
		"com.docker.compose.project.working_dir": "/clinic/install",
	}}
	var out strings.Builder
	if err := tmpl.Execute(&out, row); err != nil {
		t.Fatalf("docker ps --format would reject this template: %v", err)
	}
	if out.String() != "/clinic/install" {
		t.Fatalf("working directory = %q", out.String())
	}
}

func TestCachedImagesDoNotBlockSetup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX command fixtures")
	}
	root := t.TempDir()
	t.Setenv("HOME", root)
	docker := "#!/bin/sh\ncase \"$1 $2\" in \"images --format\") echo care:clinic ;; esac\n"
	if err := os.WriteFile(filepath.Join(root, "docker"), []byte(docker), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "security"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root)
	report, err := Scan(Options{
		Runner: proc.Runner{Env: os.Environ()}, Project: "care-desktop",
		InstallDir: filepath.Join(root, "install"), Images: []string{"care:clinic"},
	})
	if err != nil || !report.Clean || len(report.Traces) != 1 || report.Traces[0].ID != "images" {
		t.Fatalf("cached images should be reported but not block: %+v, %v", report, err)
	}
	if err := os.MkdirAll(filepath.Join(root, "install"), 0o700); err != nil {
		t.Fatal(err)
	}
	report, err = Scan(Options{
		Runner: proc.Runner{Env: os.Environ()}, Project: "care-desktop",
		InstallDir: filepath.Join(root, "install"), Images: []string{"care:clinic"},
	})
	if err != nil || report.Clean {
		t.Fatalf("installed files should still block: %+v, %v", report, err)
	}
}
