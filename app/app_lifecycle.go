package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ohcnetwork/care_desktop/app/internal/prereq"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"

	"github.com/wailsapp/wails/v2/pkg/options"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.refreshInstallDir()
	a.advStop = make(chan struct{})
	a.startAdvertise()
	go a.watchAdvertise()
	if a.loadConfig().Role != roleServer {
		return
	}
	go func() {
		a.log.Writef("docker: %s", prereq.DockerCheck(a.engine().Runner()).Message)
		prereq.EnsureRancherSettings()
	}()
}

func (a *App) refreshInstallDir() {
	updated := false
	if err := a.withJob(func() error {
		cfg := a.loadConfig()
		if cfg.Role != roleServer || !cfg.SetupDone || cfg.Removing {
			return nil
		}
		pending, err := a.engine().Backups().PendingRestore()
		if err != nil {
			return err
		}
		if pending {
			a.logln("An unfinished restore was found; its installed configuration was left unchanged for recovery.")
			return nil
		}
		if _, err := a.ensureInstallDir(); err != nil {
			return err
		}
		if err := a.engine().ApplyDomain(); err != nil {
			return err
		}
		updated = true
		return nil
	}); err != nil {
		a.logln("error: couldn't update the installed configuration (" + err.Error() + ")")
		return
	}
	if updated {
		a.logln("Install files are up to date with this version.")
	}
}

func (a *App) shutdown(context.Context) {
	if a.advStop != nil {
		close(a.advStop)
	}
	a.advMu.Lock()
	a.adv.Stop()
	a.adv = nil
	a.advMu.Unlock()
}

const (
	singleInstanceID = "ohc.care-desktop"

	quitPromptTimeout = 10 * time.Second

	stopDeadline = 90 * time.Second

	keepRunning = "Keep running"
	stopAndQuit = "Stop CARE and quit"
)

func (a *App) onSecondInstance(options.SecondInstanceData) {
	if a.ctx == nil {
		return
	}
	wruntime.WindowUnminimise(a.ctx)
	wruntime.Show(a.ctx)
}

func (a *App) beforeClose(context.Context) (prevent bool) {
	if !a.jobMu.TryLock() {
		a.logln("An operation is still running. Wait for it to finish before closing CARE Desktop.")
		return true
	}
	defer a.jobMu.Unlock()
	if a.closing {
		return false
	}
	if a.loadConfig().Role != roleServer {
		a.closing = true
		return false
	}
	answer := make(chan string, 1)
	go func() { answer <- a.askBeforeQuit() }()

	select {
	case sel := <-answer:
		switch sel {
		case keepRunning:
			return true
		case stopAndQuit:
			if err := a.stopForQuit(); err != nil {
				a.logln("error: " + err.Error())
				a.notifyActionFailed("stop", err.Error())
				return true
			}
		}
		a.closing = true
		return false
	case <-time.After(quitPromptTimeout):
		a.closing = true
		return false
	}
}

func (a *App) askBeforeQuit() string {
	if !a.clinicRunning() {
		return ""
	}
	name := a.loadConfig().MDNSName
	sel, err := wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
		Type:  wruntime.QuestionDialog,
		Title: "Quit CARE Desktop?",
		Message: "CARE will keep running in the background, but " + name +
			" will stop working for other devices on the clinic's WiFi until this app is open again.\n\n" +
			"You can also shut the clinic down completely.",
		Buttons:       []string{keepRunning, stopAndQuit},
		DefaultButton: keepRunning,
		CancelButton:  keepRunning,
	})
	if err != nil {
		return ""
	}
	return sel
}

func (a *App) clinicRunning() bool {
	cfg := a.loadConfig()
	if cfg.Role != roleServer || !cfg.SetupDone || cfg.Removing {
		return false
	}
	if _, err := os.Stat(filepath.Join(a.installDir(), "docker-compose.yml")); err != nil {
		return false
	}
	out, err := a.engine().Status()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasSuffix(strings.TrimSpace(line), " running") {
			return true
		}
	}
	return false
}

func (a *App) stopForQuit() error {
	if err := a.requireServer(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), stopDeadline)
	defer cancel()
	run := a.engine().Runner()
	cmd := proc.CommandContext(ctx, "docker", "compose", "stop")
	cmd.Dir, cmd.Env = run.Dir, run.Env
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Docker did not finish stopping; CARE Desktop was kept open: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
