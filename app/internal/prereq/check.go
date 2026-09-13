package prereq

import (
	"context"
	"runtime"
	"strings"
	"time"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

const cmdTimeout = 5 * time.Second

type Status struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func DockerCheck(run proc.Runner) Status {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	cmd := proc.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Os}}/{{.Server.Version}}")
	cmd.Env = run.Env
	out, err := cmd.Output()
	missing, stopped := dockerAdvice()
	switch {
	case err == nil:
		serverOS, version, _ := strings.Cut(strings.TrimSpace(string(out)), "/")
		if problem := wrongContainerOS(serverOS); problem != "" {
			return Status{OK: false, Message: problem}
		}
		if !hasCompose(run) {
			return Status{OK: false, Message: composeAdvice()}
		}
		return Status{OK: true, Message: "Docker " + version}
	case isNotFound(err):
		return Status{OK: false, Message: missing}
	default:
		return Status{OK: false, Message: stopped}
	}
}

func dockerAdvice() (missing, stopped string) {
	switch runtime.GOOS {
	case "windows":
		return "Docker Desktop is not installed.",
			"Docker Desktop is installed but not running."
	case "darwin":
		return "Docker Desktop is not installed.",
			"Docker Desktop is installed but not running."
	default:
		return "Docker is not installed.",
			"Docker is installed but not running."
	}
}

func composeAdvice() string {
	if runtime.GOOS == "linux" {
		return "Docker is running, but the Compose plugin is missing. Install docker-compose-plugin with your package manager."
	}
	return "Docker is running, but the Compose v2 plugin is missing. Update Docker Desktop."
}

func wrongContainerOS(serverOS string) string {
	if serverOS == "" || serverOS == "linux" {
		return ""
	}
	return "Docker Desktop is set to " + serverOS + " containers, and CARE needs Linux containers. " +
		"Right-click the Docker Desktop tray icon and choose \"Switch to Linux containers\"."
}

func hasCompose(run proc.Runner) bool {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	cmd := proc.CommandContext(ctx, "docker", "compose", "version")
	cmd.Env = run.Env
	return cmd.Run() == nil
}

func isNotFound(err error) bool {
	return strings.Contains(err.Error(), "executable file not found") ||
		strings.Contains(err.Error(), "cannot find the file")
}

func GitCheck(run proc.Runner) Status {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	cmd := proc.CommandContext(ctx, "git", "--version")
	cmd.Env = run.Env
	out, err := cmd.Output()
	if err == nil {
		return Status{OK: true, Message: strings.TrimSpace(string(out))}
	}
	return Status{OK: false, Message: "Git is not installed."}
}
