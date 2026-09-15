package prereq

import (
	"context"
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

	"github.com/ohcnetwork/care_desktop/app/internal/sys/elevate"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

type Provisioner struct {
	Log func(string)
	run proc.Runner
}

func NewProvisioner(run proc.Runner, log func(string)) *Provisioner {
	return &Provisioner{Log: log, run: run}
}

func (pr *Provisioner) logln(s string) {
	if pr.Log != nil {
		pr.Log(s)
	}
}

const (
	dockerDMGArm64 = "https://desktop.docker.com/mac/main/arm64/Docker.dmg"
	dockerDMGAmd64 = "https://desktop.docker.com/mac/main/amd64/Docker.dmg"
	dockerEXEWin   = "https://desktop.docker.com/win/main/amd64/Docker%20Desktop%20Installer.exe"
	dockerPageURL  = "https://www.docker.com/products/docker-desktop/"
	gitPageURL     = "https://git-scm.com/downloads"

	dockerReadyTimeout = 3 * time.Minute
)

type ToolAction string

const (
	ActionNone    ToolAction = ""
	ActionInstall ToolAction = "install"
	ActionOpen    ToolAction = "open"
	ActionManual  ToolAction = "manual"
)

type ToolPlan struct {
	Action ToolAction `json:"action"`
	Label  string     `json:"label"`
	Detail string     `json:"detail"`
	URL    string     `json:"url"`
}

func (pr *Provisioner) DockerPlan() ToolPlan {
	if DockerCheck(pr.run).OK {
		return ToolPlan{URL: dockerPageURL}
	}

	if !pr.dockerDaemonUp() && dockerDesktopInstalled() {
		return ToolPlan{
			Action: ActionOpen, Label: "Open " + dockerName(), URL: dockerPageURL,
			Detail: "Starts Docker and waits for it to be ready. This usually takes a minute.",
		}
	}
	return pr.dockerInstallPlan()
}

func (pr *Provisioner) dockerInstallPlan() ToolPlan {
	p := ToolPlan{
		Action: ActionInstall, Label: "Install " + dockerName(), URL: dockerPageURL,
	}
	switch runtime.GOOS {
	case "darwin":
		p.Detail = "Downloads Docker Desktop from docker.com and installs it. " +
			"You'll be asked for this Mac's password. Keep server connected to internet."
	case "windows":
		if hasCommand("winget") {
			p.Detail = "Installs Docker Desktop using Windows' own installer (winget). " +
				"Windows may need to turn on WSL 2 and restart before Docker can run."
		} else {
			p.Detail = "Installs Docker Desktop from docker.com. " +
				"Windows may need to turn on WSL 2 and restart before Docker can run."
		}
	case "linux":
		pm := linuxPackageManager()
		if pm == "" {
			p.Action, p.Label = ActionManual, "Get Docker"
			p.Detail = "Install Docker Engine and the Compose plugin with your distribution's package manager."
			return p
		}
		p.Detail = "Installs Docker Engine and the Compose plugin with " + pm +
			", then starts it. You'll be asked for your password."
	default:
		p.Action, p.Label = ActionManual, "Get Docker"
		p.Detail = "Install Docker for this system."
	}
	return p
}

func dockerName() string {
	if runtime.GOOS == "linux" {
		return "Docker"
	}
	return "Docker Desktop"
}

func (pr *Provisioner) GitPlan() ToolPlan {
	if GitCheck(pr.run).OK {
		return ToolPlan{URL: gitPageURL}
	}
	p := ToolPlan{Action: ActionInstall, Label: "Install Git", URL: gitPageURL}
	switch runtime.GOOS {
	case "darwin":
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
	default:
		p.Action, p.Label = ActionManual, "Get Git"
		p.Detail = "Install git for this system."
	}
	return p
}

func (pr *Provisioner) InstallDocker() (string, error) {
	var err error
	switch runtime.GOOS {
	case "darwin":
		err = pr.installDockerDarwin()
	case "windows":
		err = pr.installDockerWindows()
	case "linux":
		err = pr.installDockerLinux()
	default:
		return "", fmt.Errorf("installing Docker isn't supported on %s - install it from %s", runtime.GOOS, dockerPageURL)
	}
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "windows":
		return "Docker Desktop is installed.\n\nWindows may need to restart before it can run. " +
			"Start Docker Desktop, wait until it reports \"Engine running\", then choose Check again.", nil
	case "linux":
		return "Docker is installed.\n\nIf the check still fails, log out and back in so your user " +
			"picks up the docker group, then choose Check again.", nil
	default:
		return "Docker Desktop is installed.\n\nIt will start on its own; that takes about a minute. " +
			"Then choose Check again.", nil
	}
}

func (pr *Provisioner) installDockerDarwin() error {
	url := dockerDMGAmd64
	if runtime.GOARCH == "arm64" {
		url = dockerDMGArm64
	}
	dmg, err := pr.download(url, "Docker.dmg")
	if err != nil {
		return err
	}
	defer os.Remove(dmg)

	const mount = "/Volumes/Docker"
	pr.logln("Installing Docker Desktop. macOS will ask for your password...")
	sh := strings.Join([]string{
		"hdiutil attach -nobrowse " + elevate.ShQuote(dmg),
		elevate.ShQuote(mount+"/Docker.app/Contents/MacOS/install") +
			" --accept-license --user=" + elevate.ShQuote(currentUsername()),
		"hdiutil detach " + elevate.ShQuote(mount),
	}, " && ")
	if err := elevate.Run(sh, true); err != nil {
		_ = proc.Command("hdiutil", "detach", mount).Run() // never leave the image mounted
		return fmt.Errorf("could not install Docker Desktop: %w", err)
	}
	pr.logln("Docker Desktop installed.")
	return pr.OpenDocker()
}

func (pr *Provisioner) installDockerWindows() error {
	if hasCommand("winget") {
		pr.logln("Installing Docker Desktop with winget...")
		err := pr.run.Run("winget", "install", "-e", "--id", "Docker.DockerDesktop",
			"--accept-package-agreements", "--accept-source-agreements")
		if err == nil {
			return pr.afterWindowsDockerInstall()
		}
		pr.logln("winget couldn't install it; falling back to the installer from docker.com.")
	}
	exe, err := pr.download(dockerEXEWin, "DockerDesktopInstaller.exe")
	if err != nil {
		return err
	}
	defer os.Remove(exe)
	pr.logln("Running the Docker Desktop installer. Windows will ask for permission...")
	if err := pr.runElevated(exe, "install", "--quiet", "--accept-license", "--backend=wsl-2"); err != nil {
		return fmt.Errorf("could not install Docker Desktop: %w", err)
	}
	return pr.afterWindowsDockerInstall()
}

func (pr *Provisioner) afterWindowsDockerInstall() error {
	pr.logln("Docker Desktop installed.")
	if err := pr.OpenDocker(); err != nil {
		return fmt.Errorf("Docker Desktop is installed but didn't start. "+
			"Windows may need to restart to finish turning on WSL 2 - restart, "+
			"open Docker Desktop, then run the check again (%w)", err)
	}
	return nil
}

func (pr *Provisioner) installDockerLinux() error {
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
	sh := install +
		" && systemctl enable --now docker" +
		" && usermod -aG docker " + elevate.ShQuote(currentUsername())
	pr.logln("Installing Docker Engine with " + pm + "...")
	if err := elevate.Run(sh, true); err != nil {
		return fmt.Errorf("could not install Docker: %w", err)
	}
	pr.logln("Docker installed. If the check below still fails, log out and back in " +
		"so this account picks up its new 'docker' group membership.")
	return pr.waitForDocker(30 * time.Second)
}

func (pr *Provisioner) InstallGit() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		pr.logln("Asking macOS to install its command line tools (this includes Git)...")
		if err := pr.run.Run("xcode-select", "--install"); err != nil {
			return "", fmt.Errorf("could not start the macOS command line tools installer "+
				"(it may already be installing): %w", err)
		}
		return "macOS is installing its command line tools, which include Git.\n\n" +
			"Choose Install in the window macOS just opened and wait for it to finish, " +
			"then choose Check again.", nil
	case "windows":
		if !hasCommand("winget") {
			return "", fmt.Errorf("winget isn't available - install Git from %s", gitPageURL)
		}
		pr.logln("Installing Git with winget...")
		if err := pr.run.Run("winget", "install", "-e", "--id", "Git.Git",
			"--accept-package-agreements", "--accept-source-agreements"); err != nil {
			return "", err
		}
		return "Git is installed.\n\nChoose Check again to continue.", nil
	case "linux":
		pm := linuxPackageManager()
		if pm == "" {
			return "", fmt.Errorf("no supported package manager found - install git from %s", gitPageURL)
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
		pr.logln("Installing git with " + pm + "...")
		if err := elevate.Run(sh, true); err != nil {
			return "", err
		}
		return "Git is installed.\n\nChoose Check again to continue.", nil
	}
	return "", fmt.Errorf("installing git isn't supported on %s - install it from %s", runtime.GOOS, gitPageURL)
}

func (pr *Provisioner) OpenDocker() error {
	pr.logln("Starting Docker...")
	switch runtime.GOOS {
	case "darwin":
		if err := pr.run.Run("open", "-a", "Docker"); err != nil {
			return fmt.Errorf("could not start Docker Desktop: %w", err)
		}
	case "windows":
		exe := windowsDockerDesktopExe()
		if exe == "" {
			return fmt.Errorf("Docker Desktop is not installed")
		}
		if err := pr.run.Run("powershell", "-NoProfile", "-Command",
			"Start-Process "+elevate.PSQuote(exe)); err != nil {
			return fmt.Errorf("could not start Docker Desktop: %w", err)
		}
	case "linux":
		if err := elevate.Run("systemctl start docker", true); err != nil {
			return fmt.Errorf("could not start the Docker service: %w", err)
		}
	}
	return pr.waitForDocker(dockerReadyTimeout)
}

func (pr *Provisioner) waitForDocker(limit time.Duration) error {
	deadline := time.Now().Add(limit)
	for {
		if DockerCheck(pr.run).OK {
			pr.logln("Docker is ready.")
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("Docker didn't become ready in time - open it yourself, " +
				"wait until it says it is running, then run the check again")
		}
		time.Sleep(3 * time.Second)
	}
}

func (pr *Provisioner) dockerDaemonUp() bool {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	cmd := proc.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}")
	cmd.Env = pr.run.Env
	return cmd.Run() == nil
}

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

func (pr *Provisioner) runElevated(exe string, args ...string) error {
	if runtime.GOOS == "windows" {
		ps := "$p = Start-Process " + elevate.PSQuote(exe) + " -Wait -PassThru -Verb RunAs"
		if len(args) > 0 {
			quoted := make([]string, 0, len(args))
			for _, a := range args {
				quoted = append(quoted, elevate.PSQuote(a))
			}
			ps += " -ArgumentList " + strings.Join(quoted, ",")
		}
		ps += "; exit $p.ExitCode"
		return proc.Command("powershell", "-NoProfile", "-Command", ps).Run()
	}
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, elevate.ShQuote(exe))
	for _, a := range args {
		parts = append(parts, elevate.ShQuote(a))
	}
	return elevate.Run(strings.Join(parts, " "), true)
}

const downloadHeaderTimeout = 30 * time.Second

var downloadStallTimeout = 2 * time.Minute

func downloadClient() *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.ResponseHeaderTimeout = downloadHeaderTimeout
	return &http.Client{Transport: tr}
}

func (pr *Provisioner) download(url, name string) (string, error) {
	pr.logln("Downloading " + name + " from " + hostOf(url) + "...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stall := time.AfterFunc(downloadStallTimeout, cancel)
	defer stall.Stop()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := downloadClient().Do(req)
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
		log:   pr.logln,
		name:  name,
		alive: func() { stall.Reset(downloadStallTimeout) },
	})
	closeErr := f.Close()
	if err != nil {
		os.Remove(path)
		if ctx.Err() != nil {
			return "", fmt.Errorf("the download of %s stopped making progress for %s - "+
				"check this computer's internet connection and try again", name, downloadStallTimeout)
		}
		return "", fmt.Errorf("could not download %s: %w", name, err)
	}
	if closeErr != nil {
		os.Remove(path)
		return "", closeErr
	}
	pr.logln("Downloaded " + name + ".")
	return path, nil
}

type progressReader struct {
	r        io.Reader
	total    int64
	read     int64
	lastStep int64
	log      func(string)
	name     string
	alive    func()
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.read += int64(n)
	if n > 0 && p.alive != nil {
		p.alive()
	}
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
