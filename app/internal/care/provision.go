package care

// Fetching the two prerequisites the app can't bundle. A clinic operator should
// never have to open a terminal or judge which Docker to download, so every
// "not ready" row in the wizard carries the action that fixes it: install it,
// or start it.
//
// Sources are the vendors' own, and every URL here is from their install docs:
// https://docs.docker.com/desktop/setup/install/mac-install/
// https://docs.docker.com/desktop/setup/install/windows-install/

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	dockerDMGArm64 = "https://desktop.docker.com/mac/main/arm64/Docker.dmg"
	dockerDMGAmd64 = "https://desktop.docker.com/mac/main/amd64/Docker.dmg"
	dockerEXEWin   = "https://desktop.docker.com/win/main/amd64/Docker%20Desktop%20Installer.exe"
	dockerPageURL  = "https://www.docker.com/products/docker-desktop/"
	gitPageURL     = "https://git-scm.com/downloads"

	dockerReadyTimeout = 3 * time.Minute
)

// ToolAction is what the app can offer for a prerequisite that isn't ready.
type ToolAction string

const (
	ActionNone    ToolAction = ""        // already fine, nothing to offer
	ActionInstall ToolAction = "install" // we can fetch and install it here
	ActionOpen    ToolAction = "open"    // installed but not running
	ActionManual  ToolAction = "manual"  // we can only open the download page
)

// ToolPlan is the wizard's instruction for one prerequisite: which button to
// show, what it will do, and whether it will ask for a password.
type ToolPlan struct {
	Tool       string     `json:"tool"`
	Action     ToolAction `json:"action"`
	Label      string     `json:"label"`
	Detail     string     `json:"detail"`
	NeedsAdmin bool       `json:"needs_admin"`
	URL        string     `json:"url"` // where to send them if we can't do it
}

// --- plans ------------------------------------------------------------------

// DockerPlan decides what to offer for Docker on this machine.
func (e *Engine) DockerPlan() ToolPlan {
	if e.DockerCheck().OK {
		return ToolPlan{Tool: "docker", URL: dockerPageURL}
	}
	// Installed but asleep is the common case, and much cheaper to fix than a
	// re-download - offer that before offering an install.
	if !e.dockerDaemonUp() && dockerDesktopInstalled() {
		return ToolPlan{
			Tool: "docker", Action: ActionOpen, Label: "Open Docker", URL: dockerPageURL,
			Detail:     "Starts Docker and waits for it to be ready. This usually takes a minute.",
			NeedsAdmin: runtime.GOOS == "linux", // only there is starting the daemon privileged
		}
	}
	return e.dockerInstallPlan()
}

func (e *Engine) dockerInstallPlan() ToolPlan {
	p := ToolPlan{
		Tool: "docker", Action: ActionInstall, Label: "Install Docker",
		URL: dockerPageURL, NeedsAdmin: true,
	}
	switch runtime.GOOS {
	case "darwin":
		p.Detail = "Downloads Docker Desktop from docker.com and installs it. " +
			"You'll be asked for this Mac's password. Allow 10 minutes on a slow connection."
	case "windows":
		p.Detail = "Installs Docker Desktop from docker.com. " +
			"Windows may need to turn on WSL 2 and restart before Docker can run."
		if hasCommand("winget") {
			p.Detail = "Installs Docker Desktop using Windows' own installer (winget). " +
				"Windows may need to turn on WSL 2 and restart before Docker can run."
		}
	case "linux":
		pm := linuxPackageManager()
		if pm == "" {
			p.Action, p.Label = ActionManual, "Get Docker"
			p.Detail = "Install Docker Engine and the Compose plugin with your distribution's package manager."
			p.NeedsAdmin = false
			return p
		}
		p.Detail = "Installs Docker Engine and the Compose plugin with " + pm +
			", then starts it. You'll be asked for your password."
	default:
		p.Action, p.Label, p.NeedsAdmin = ActionManual, "Get Docker", false
		p.Detail = "Install Docker for this system."
	}
	return p
}

// GitPlan decides what to offer for git. Git is never "running", so the only
// outcomes are install or a download page.
func (e *Engine) GitPlan() ToolPlan {
	if e.GitCheck().OK {
		return ToolPlan{Tool: "git", URL: gitPageURL}
	}
	p := ToolPlan{Tool: "git", Action: ActionInstall, Label: "Install Git", URL: gitPageURL}
	switch runtime.GOOS {
	case "darwin":
		// Apple ships git inside the Command Line Tools, so the supported route
		// is the OS installer rather than a download of our own.
		p.Detail = "Asks macOS to install its developer command line tools, which include Git. " +
			"A system window will appear - choose Install."
	case "windows":
		if !hasCommand("winget") {
			p.Action, p.Label = ActionManual, "Get Git"
			p.Detail = "Download and install Git for Windows."
			return p
		}
		p.Detail = "Installs Git using Windows' own installer (winget)."
	case "linux":
		pm := linuxPackageManager()
		if pm == "" {
			p.Action, p.Label = ActionManual, "Get Git"
			p.Detail = "Install git with your distribution's package manager."
			return p
		}
		p.Detail = "Installs git with " + pm + ". You'll be asked for your password."
		p.NeedsAdmin = true
	default:
		p.Action, p.Label = ActionManual, "Get Git"
		p.Detail = "Install git for this system."
	}
	return p
}

// --- installs ---------------------------------------------------------------

func (e *Engine) InstallDocker() error {
	switch runtime.GOOS {
	case "darwin":
		return e.installDockerDarwin()
	case "windows":
		return e.installDockerWindows()
	case "linux":
		return e.installDockerLinux()
	}
	return fmt.Errorf("installing Docker isn't supported on %s - install it from %s", runtime.GOOS, dockerPageURL)
}

func (e *Engine) installDockerDarwin() error {
	url := dockerDMGAmd64
	if runtime.GOARCH == "arm64" {
		url = dockerDMGArm64
	}
	dmg, err := e.download(url, "Docker.dmg")
	if err != nil {
		return err
	}
	defer os.Remove(dmg)

	const mount = "/Volumes/Docker"
	e.logln("Installing Docker Desktop. macOS will ask for your password...")
	// One elevated shell for all three steps: split up, it would prompt for the
	// password three times. --user pre-grants the privileged bits so Docker's
	// own first run doesn't ask again.
	sh := strings.Join([]string{
		"hdiutil attach -nobrowse " + shSingleQuote(dmg),
		shSingleQuote(mount+"/Docker.app/Contents/MacOS/install") +
			" --accept-license --user=" + shSingleQuote(currentUsername()),
		"hdiutil detach " + shSingleQuote(mount),
	}, " && ")
	if err := e.runPrivileged(sh, true); err != nil {
		_ = newCmd("hdiutil", "detach", mount).Run() // never leave the image mounted
		return fmt.Errorf("could not install Docker Desktop: %w", err)
	}
	e.logln("Docker Desktop installed.")
	return e.OpenDocker()
}

func (e *Engine) installDockerWindows() error {
	if hasCommand("winget") {
		e.logln("Installing Docker Desktop with winget...")
		err := e.run(nil, "winget", "install", "-e", "--id", "Docker.DockerDesktop",
			"--accept-package-agreements", "--accept-source-agreements")
		if err == nil {
			return e.afterWindowsDockerInstall()
		}
		e.logln("winget couldn't install it; falling back to the installer from docker.com.")
	}
	exe, err := e.download(dockerEXEWin, "DockerDesktopInstaller.exe")
	if err != nil {
		return err
	}
	defer os.Remove(exe)
	e.logln("Running the Docker Desktop installer. Windows will ask for permission...")
	if err := e.runElevated(exe, "install", "--quiet", "--accept-license", "--backend=wsl-2"); err != nil {
		return fmt.Errorf("could not install Docker Desktop: %w", err)
	}
	return e.afterWindowsDockerInstall()
}

// Docker Desktop needs WSL 2, and turning that on takes a restart that no
// installer can skip. Rather than fail the wizard, say so plainly.
func (e *Engine) afterWindowsDockerInstall() error {
	e.logln("Docker Desktop installed.")
	if err := e.OpenDocker(); err != nil {
		return fmt.Errorf("Docker Desktop is installed but didn't start. "+
			"Windows may need to restart to finish turning on WSL 2 - restart, "+
			"open Docker Desktop, then run the check again (%w)", err)
	}
	return nil
}

func (e *Engine) installDockerLinux() error {
	pm := linuxPackageManager()
	if pm == "" {
		return fmt.Errorf("no supported package manager found - install Docker from %s", dockerPageURL)
	}
	var install string
	switch pm {
	case "apt":
		install = "apt-get update && apt-get install -y docker.io docker-compose-plugin"
	case "dnf":
		install = "dnf install -y docker docker-compose-plugin"
	case "zypper":
		install = "zypper --non-interactive install docker docker-compose"
	case "pacman":
		install = "pacman -Sy --noconfirm docker docker-compose"
	}
	// Group membership is what lets the app reach the socket without sudo. It
	// only takes effect on the next login, which is why it is called out below.
	sh := install +
		" && systemctl enable --now docker" +
		" && usermod -aG docker " + shSingleQuote(currentUsername())
	e.logln("Installing Docker Engine with " + pm + "...")
	if err := e.runPrivileged(sh, true); err != nil {
		return fmt.Errorf("could not install Docker: %w", err)
	}
	e.logln("Docker installed. If the check below still fails, log out and back in " +
		"so this account picks up its new 'docker' group membership.")
	return e.waitForDocker(30 * time.Second)
}

func (e *Engine) InstallGit() error {
	switch runtime.GOOS {
	case "darwin":
		// Returns as soon as the system dialog is on screen, so there is nothing
		// to wait for here - the operator finishes it and re-runs the check.
		e.logln("Asking macOS to install its command line tools (this includes Git)...")
		if err := e.run(nil, "xcode-select", "--install"); err != nil {
			return fmt.Errorf("could not start the macOS command line tools installer "+
				"(it may already be installing): %w", err)
		}
		e.logln("Choose Install in the window macOS just opened, then run the check again.")
		return nil
	case "windows":
		if !hasCommand("winget") {
			return fmt.Errorf("winget isn't available - install Git from %s", gitPageURL)
		}
		e.logln("Installing Git with winget...")
		return e.run(nil, "winget", "install", "-e", "--id", "Git.Git",
			"--accept-package-agreements", "--accept-source-agreements")
	case "linux":
		pm := linuxPackageManager()
		if pm == "" {
			return fmt.Errorf("no supported package manager found - install git from %s", gitPageURL)
		}
		var sh string
		switch pm {
		case "apt":
			sh = "apt-get update && apt-get install -y git"
		case "dnf":
			sh = "dnf install -y git"
		case "zypper":
			sh = "zypper --non-interactive install git"
		case "pacman":
			sh = "pacman -Sy --noconfirm git"
		}
		e.logln("Installing git with " + pm + "...")
		return e.runPrivileged(sh, true)
	}
	return fmt.Errorf("installing git isn't supported on %s - install it from %s", runtime.GOOS, gitPageURL)
}

// --- starting Docker ---------------------------------------------------------

// OpenDocker starts the engine and waits for it to answer, so the wizard can go
// green on its own rather than telling the operator to watch a tray icon.
func (e *Engine) OpenDocker() error {
	e.logln("Starting Docker...")
	switch runtime.GOOS {
	case "darwin":
		if err := e.run(nil, "open", "-a", "Docker"); err != nil {
			return fmt.Errorf("could not start Docker Desktop: %w", err)
		}
	case "windows":
		exe := windowsDockerDesktopExe()
		if exe == "" {
			return fmt.Errorf("Docker Desktop is not installed")
		}
		if err := e.run(nil, "powershell", "-NoProfile", "-Command",
			"Start-Process "+psSingleQuote(exe)); err != nil {
			return fmt.Errorf("could not start Docker Desktop: %w", err)
		}
	case "linux":
		if err := e.runPrivileged("systemctl start docker", true); err != nil {
			return fmt.Errorf("could not start the Docker service: %w", err)
		}
	}
	return e.waitForDocker(dockerReadyTimeout)
}

func (e *Engine) waitForDocker(limit time.Duration) error {
	deadline := time.Now().Add(limit)
	for {
		if e.DockerCheck().OK {
			e.logln("Docker is ready.")
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("Docker didn't become ready in time - open it yourself, " +
				"wait until it says it is running, then run the check again")
		}
		time.Sleep(3 * time.Second)
	}
}

// --- detection ---------------------------------------------------------------

// dockerDaemonUp reports whether a daemon answers, regardless of whether the
// Compose plugin is also present.
func (e *Engine) dockerDaemonUp() bool {
	cmd := newCmd("docker", "version", "--format", "{{.Server.Version}}")
	cmd.Env = e.baseEnv()
	return cmd.Run() == nil
}

// dockerDesktopInstalled looks for the application rather than the CLI: on macOS
// and Windows the `docker` binary arrives with Docker Desktop, so an app present
// with no daemon means "installed but not started".
func dockerDesktopInstalled() bool {
	switch runtime.GOOS {
	case "darwin":
		_, err := os.Stat("/Applications/Docker.app")
		return err == nil
	case "windows":
		return windowsDockerDesktopExe() != ""
	default:
		return hasCommand("docker")
	}
}

func windowsDockerDesktopExe() string {
	for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramW6432"), `C:\Program Files`} {
		if base == "" {
			continue
		}
		p := filepath.Join(base, "Docker", "Docker", "Docker Desktop.exe")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// linuxPackageManager returns the manager we know how to drive, or "".
func linuxPackageManager() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	for _, pm := range []string{"apt", "dnf", "zypper", "pacman"} {
		lookup := pm
		if pm == "apt" {
			lookup = "apt-get"
		}
		if hasCommand(lookup) {
			return pm
		}
	}
	return ""
}

func currentUsername() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return os.Getenv("USER")
}

// --- plumbing ----------------------------------------------------------------

// runElevated runs one program behind the OS privilege prompt.
func (e *Engine) runElevated(exe string, args ...string) error {
	if runtime.GOOS == "windows" {
		ps := "Start-Process " + psSingleQuote(exe) + " -Wait -Verb RunAs"
		if len(args) > 0 {
			quoted := make([]string, 0, len(args))
			for _, a := range args {
				quoted = append(quoted, psSingleQuote(a))
			}
			ps += " -ArgumentList " + strings.Join(quoted, ",")
		}
		return newCmd("powershell", "-NoProfile", "-Command", ps).Run()
	}
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, shSingleQuote(exe))
	for _, a := range args {
		parts = append(parts, shSingleQuote(a))
	}
	return e.runPrivileged(strings.Join(parts, " "), true)
}

// download fetches to a temp file, logging progress: these are hundreds of
// megabytes on connections that can make that take a while, and a wizard that
// looks frozen gets closed.
func (e *Engine) download(url, name string) (string, error) {
	e.logln("Downloading " + name + " from " + hostOf(url) + "...")
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("could not download %s: %w", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("could not download %s: the server said %s", name, resp.Status)
	}

	f, err := os.CreateTemp("", "care-*-"+name)
	if err != nil {
		return "", err
	}
	path := f.Name()
	_, err = io.Copy(f, &progressReader{
		r:     resp.Body,
		total: resp.ContentLength,
		log:   e.logln,
		name:  name,
	})
	closeErr := f.Close()
	if err != nil {
		os.Remove(path)
		return "", fmt.Errorf("could not download %s: %w", name, err)
	}
	if closeErr != nil {
		os.Remove(path)
		return "", closeErr
	}
	e.logln("Downloaded " + name + ".")
	return path, nil
}

type progressReader struct {
	r        io.Reader
	total    int64
	read     int64
	lastStep int64
	log      func(string)
	name     string
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.read += int64(n)
	if p.total > 0 {
		if step := p.read * 10 / p.total; step > p.lastStep {
			p.lastStep = step
			p.log(fmt.Sprintf("  %s: %d%%", p.name, step*10))
		}
	}
	return n, err
}

func hostOf(rawURL string) string {
	s := strings.TrimPrefix(strings.TrimPrefix(rawURL, "https://"), "http://")
	if i := strings.IndexByte(s, '/'); i >= 0 {
		return s[:i]
	}
	return s
}
