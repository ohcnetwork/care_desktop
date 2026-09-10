package main

import (
	"errors"
	"os"
	"strings"
	"syscall"

	"github.com/ohcnetwork/care_desktop/app/internal/health"
	"github.com/ohcnetwork/care_desktop/app/internal/prereq"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/mdns"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/netfix"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/reboot"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/crypto/bcrypt"
)

// --- state the installer/panel read on load ---------------------------------

type AppState struct {
	SetupDone bool          `json:"setup_done"`
	MDNSName  string        `json:"mdns_name"`
	Docker    prereq.Status `json:"docker"`
}

func (a *App) GetState() AppState {
	cfg := a.loadConfig()
	return AppState{
		SetupDone: cfg.SetupDone,
		MDNSName:  cfg.MDNSName,
		Docker:    prereq.DockerCheck(a.engine().Runner()),
	}
}

func (a *App) DockerStatus() prereq.Status { return prereq.DockerCheck(a.engine().Runner()) }
func (a *App) GitStatus() prereq.Status    { return prereq.GitCheck(a.engine().Runner()) }
func (a *App) CareHealth() health.Health   { return health.Ping(a.engine().Host()) }

// NetworkStatus flags a Public Windows profile (blocks LAN discovery of care.local).
func (a *App) NetworkStatus() netfix.Status { return netfix.Check(a.engine().Runner()) }

// FixNetwork sets the network Private and opens the clinic's ports (elevated).
func (a *App) FixNetwork() error { return netfix.Fix(a.engine().Log) }

// DockerPlan / GitPlan tell the wizard which button to put on a failing
// prerequisite row - install it, start it, or just open the download page.
func (a *App) DockerPlan() prereq.ToolPlan { return a.provisioner().DockerPlan() }
func (a *App) GitPlan() prereq.ToolPlan    { return a.provisioner().GitPlan() }

// InstallDocker, InstallGit and OpenDocker carry out those plans. They stream
// progress through care-log like the install does, and block until finished, so
// the wizard can re-run its checks the moment they return.
func (a *App) InstallDocker() error { return a.provisioner().InstallDocker() }
func (a *App) InstallGit() error    { return a.provisioner().InstallGit() }
func (a *App) OpenDocker() error    { return a.provisioner().OpenDocker() }

// provisioner streams its progress through care-log, like the install does.
func (a *App) provisioner() *prereq.Provisioner {
	e := a.engine()
	return prereq.NewProvisioner(e.Runner(), e.InstallDir, e.Log, e.Confirm)
}

// RestartPlan reports whether the machine has to restart before the
// prerequisites will work. Only meaningful straight after an install - a restart
// Windows wanted for its own reasons is not ours to nag about.
func (a *App) RestartPlan() reboot.Plan { return reboot.Check() }

// RestartNow restarts the computer, having first made the app open again by
// itself. Without the login item the operator comes back to a finished restart
// and no sign of the half-done setup that caused it.
func (a *App) RestartNow() error {
	if err := a.SetAutostart(true); err != nil {
		wruntime.EventsEmit(a.ctx, "care-log",
			"note: couldn't set CARE Desktop to open after the restart ("+err.Error()+
				") - open it yourself once the computer is back")
	}
	return reboot.Now()
}

// ValidatePassword lets the wizard check the admin password live as the user types.
// Returns "" when acceptable, otherwise a human-readable reason to show under the field.
func (a *App) ValidatePassword(pw string) string {
	if err := ValidatePassword(pw); err != nil {
		return err.Error()
	}
	return ""
}

// ValidateDomain lets the wizard check the clinic address live as the user types.
func (a *App) ValidateDomain(name string) string {
	if err := mdns.ValidateLabel(name); err != nil {
		return err.Error()
	}
	return ""
}

// ValidateBackupDir reports why dir can't hold the backups, or "" if it can.
// The test is a real write. A read-only disk image, a USB stick macOS mounted
// read-only because it is NTFS, and a plain permissions problem are
// indistinguishable from a stat, and all three would otherwise only surface as
// a failed install, once the engine tries to mkdir the backup folder.
// An empty dir means "use the default", which lives under the home dir.
func (a *App) ValidateBackupDir(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	info, err := os.Stat(dir)
	switch {
	case os.IsNotExist(err):
		return "That folder isn't there any more. If it is on a removable drive, plug the drive in and choose it again."
	case err != nil:
		return "Couldn't open that folder: " + err.Error()
	case !info.IsDir():
		return "That is a file, not a folder. Choose a folder to keep the backups in."
	}
	probe, err := os.CreateTemp(dir, ".care-write-test-*")
	if err != nil {
		switch {
		case errors.Is(err, syscall.EROFS):
			return "That drive is read-only, so backups can't be written to it. Disk images and USB drives formatted for Windows (NTFS) are read-only on this Mac. Choose a different folder."
		case errors.Is(err, os.ErrPermission):
			return "This computer isn't allowed to write to that folder. Choose a different folder."
		default:
			return "Couldn't write to that folder: " + err.Error()
		}
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return ""
}

// SetMDNSName saves the chosen address and re-advertises under it, so the
// installer's name check tests the name actually picked.
func (a *App) SetMDNSName(name string) error {
	if err := mdns.ValidateLabel(name); err != nil {
		return err
	}
	cfg := a.loadConfig()
	cfg.MDNSName = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".local") + ".local"
	if err := a.saveConfig(cfg); err != nil {
		return err
	}
	a.restartAdvertise()
	return nil
}

// VerifyAdminPassword gates the Advanced screen. It checks against the bcrypt hash
// stored at setup. Legacy installs (set up before the hash existed) have none, so
// any non-empty entry passes - a speed bump, not verification.
func (a *App) VerifyAdminPassword(pw string) bool {
	h := a.loadConfig().AdminPwHash
	if h == "" {
		return strings.TrimSpace(pw) != ""
	}
	return bcrypt.CompareHashAndPassword([]byte(h), []byte(pw)) == nil
}

// MDNSStatus is green whenever this app is actively advertising the name (advertise
// mode), since that's exactly what makes care.local resolve. Otherwise it falls back
// to the engine's resolution test + mode-specific guidance.
func (a *App) MDNSStatus() mdns.NameStatus {
	if a.advRunning() {
		full := mdns.NameStatus{OK: true}
		full.Message = a.loadConfig().MDNSName + " is being advertised by this app"
		return full
	}
	e := a.engine()
	return mdns.Check(mdns.Label(e.Label()))
}
