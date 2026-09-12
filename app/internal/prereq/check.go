package prereq

import (
	"runtime"
	"strings"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

type Status struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// DockerCheck is the one prerequisite the app can't bundle. The test is
// functional, never brand-based: whatever provides `docker` plus the `docker
// compose` v2 plugin is accepted. Only the advice is per-OS, because what to go
// and install genuinely differs - see dockerAdvice.
func DockerCheck(run proc.Runner) Status {
	// Server.Os comes back in the same call: on Windows the daemon can be pointed
	// at Windows containers, and knowing that early is worth the extra field.
	cmd := proc.Command("docker", "version", "--format", "{{.Server.Os}}/{{.Server.Version}}")
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
		return "Docker Desktop is not installed. Install Docker Desktop with the WSL 2 backend, then start it.",
			"Docker Desktop is installed but not running. Start it and wait until it reports \"Engine running\"."
	case "darwin":
		return "Docker not found. Install a Docker engine - Docker Desktop, OrbStack or Colima - and start it.",
			"Docker is installed but not running. Start Docker Desktop or OrbStack, or run: colima start"
	default:
		return "Docker not found. Install Docker Engine plus the Compose v2 plugin, then start the service.",
			"Docker is installed but not running. Start it with: sudo systemctl start docker"
	}
}

func composeAdvice() string {
	if runtime.GOOS == "linux" {
		return "Docker is running, but the Compose plugin is missing. Install it: sudo apt install docker-compose-plugin (or the docker-compose-plugin package for your distro)."
	}
	return "Docker is running, but the Compose v2 plugin is missing. Update Docker Desktop, or install the docker-compose plugin for your engine."
}

// wrongContainerOS catches a Windows daemon switched to Windows containers, where
// none of CARE's Linux images can run. Left to itself it surfaces much later as
// "no matching manifest for windows/amd64", which names nothing the operator can
// act on. Empty serverOS means an engine too old to report it - not worth failing.
func wrongContainerOS(serverOS string) string {
	if serverOS == "" || serverOS == "linux" {
		return ""
	}
	return "Docker is set to " + serverOS + " containers, and CARE needs Linux containers. " +
		"Right-click the Docker tray icon and choose \"Switch to Linux containers\"."
}

// hasCompose reports whether the `docker compose` v2 plugin is available - often
// missing on non-Desktop installs, and the stack can't come up without it.
func hasCompose(run proc.Runner) bool {
	cmd := proc.Command("docker", "compose", "version")
	cmd.Env = run.Env
	return cmd.Run() == nil
}

func isNotFound(err error) bool {
	return strings.Contains(err.Error(), "executable file not found") ||
		strings.Contains(err.Error(), "cannot find the file")
}

func GitCheck(run proc.Runner) Status {
	cmd := proc.Command("git", "--version")
	cmd.Env = run.Env
	out, err := cmd.Output()
	if err == nil {
		return Status{OK: true, Message: strings.TrimSpace(string(out))}
	}
	return Status{OK: false, Message: "Git not found - install Git (Git for Windows / Xcode CLT / apt-get git)."}
}
