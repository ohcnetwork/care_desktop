package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ohcnetwork/care_desktop/app/internal/prereq"

	"github.com/wailsapp/wails/v2/pkg/options"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.refreshInstallDir()
	a.advStop = make(chan struct{})
	a.startAdvertise()
	go a.watchAdvertise()
	go a.log.Writef("docker: %s", prereq.DockerCheck(a.engine().Runner()).Message)
}

func (a *App) refreshInstallDir() {
	if !loadConfig().SetupDone {
		return
	}
	if _, err := a.ensureInstallDir(); err != nil {
		a.logln("note: couldn't update the install files for this version (" + err.Error() + ")")
		return
	}
	if err := a.engine().ApplyDomain(); err != nil {
		a.logln("note: couldn't re-apply the clinic address after updating (" + err.Error() + ")")
		return
	}
	a.logln("Install files are up to date with this version.")
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
	answer := make(chan string, 1)
	go func() { answer <- a.askBeforeQuit() }()

	select {
	case sel := <-answer:
		switch sel {
		case keepRunning:
			return true
		case stopAndQuit:
			a.stopForQuit()
		}
		return false
	case <-time.After(quitPromptTimeout):
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
	if !a.loadConfig().SetupDone {
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

func (a *App) stopForQuit() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = a.engine().Stop()
	}()
	select {
	case <-done:
	case <-time.After(stopDeadline):
		a.logln("Docker did not finish stopping in time - quitting anyway.")
	}
}
