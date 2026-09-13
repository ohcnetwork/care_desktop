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

	"golang.org/x/crypto/bcrypt"
)

type AppState struct {
	Version   string        `json:"version"`
	SetupDone bool          `json:"setup_done"`
	MDNSName  string        `json:"mdns_name"`
	Docker    prereq.Status `json:"docker"`
}

func (a *App) GetState() AppState {
	cfg := a.loadConfig()
	return AppState{
		Version:   a.pins.AppVersion,
		SetupDone: cfg.SetupDone,
		MDNSName:  cfg.MDNSName,
		Docker:    prereq.DockerCheck(a.engine().Runner()),
	}
}

func (a *App) DockerStatus() prereq.Status { return prereq.DockerCheck(a.engine().Runner()) }
func (a *App) GitStatus() prereq.Status    { return prereq.GitCheck(a.engine().Runner()) }
func (a *App) ClinicHealth() health.Health { return health.Ping() }

func (a *App) NetworkStatus() netfix.Status { return netfix.Check(a.engine().Runner()) }

func (a *App) FixNetwork() error { return netfix.Fix(a.engine().Log) }

func (a *App) DockerPlan() prereq.ToolPlan { return a.provisioner().DockerPlan() }
func (a *App) GitPlan() prereq.ToolPlan    { return a.provisioner().GitPlan() }

func (a *App) InstallDocker() (string, error) { return a.provisioner().InstallDocker() }
func (a *App) InstallGit() (string, error)    { return a.provisioner().InstallGit() }
func (a *App) OpenDocker() error              { return a.provisioner().OpenDocker() }

func (a *App) provisioner() *prereq.Provisioner {
	e := a.engine()
	return prereq.NewProvisioner(e.Runner(), e.InstallDir, e.Log, e.Confirm)
}

func (a *App) RestartPlan() reboot.Plan { return reboot.Check() }

func (a *App) RestartNow() error {
	if err := a.SetAutostart(true); err != nil {
		a.logln("note: couldn't set CARE Desktop to open after the restart (" + err.Error() +
			") - open it yourself once the computer is back")
	}
	return reboot.Now()
}

func (a *App) ValidatePassword(pw string) string {
	if err := ValidatePassword(pw); err != nil {
		return err.Error()
	}
	return ""
}

func (a *App) ValidateDomain(name string) string {
	if err := mdns.ValidateLabel(name); err != nil {
		return err.Error()
	}
	return ""
}

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

func (a *App) SetMDNSName(name string) error {
	if err := mdns.ValidateLabel(name); err != nil {
		return err
	}
	cfg := a.loadConfig()
	cfg.MDNSName = mdns.Label(name) + ".local"
	if err := a.saveConfig(cfg); err != nil {
		return err
	}
	a.restartAdvertise()
	return nil
}

func (a *App) VerifyAdminPassword(pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(a.loadConfig().AdminPwHash), []byte(pw)) == nil
}

func (a *App) MDNSStatus() mdns.NameStatus {
	name := a.loadConfig().MDNSName
	switch {
	case name == "":
		return mdns.NameStatus{OK: false, Message: "No clinic address chosen yet"}
	case a.advRunning():
		return mdns.NameStatus{OK: true, Message: name + " is being advertised by this app"}
	}
	return mdns.NameStatus{OK: false, Message: "Not advertising " + name}
}
