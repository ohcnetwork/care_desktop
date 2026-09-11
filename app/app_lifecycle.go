package main

import (
	"context"
	"strings"
	"time"

	"github.com/ohcnetwork/care_desktop/app/internal/prereq"

	"github.com/wailsapp/wails/v2/pkg/options"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.advStop = make(chan struct{})
	a.startAdvertise()
	go a.watchAdvertise()
	// Off the startup path: probing Docker spawns a process and the window should
	// not wait on it. Completes the log header begun in main.
	go a.log.Writef("docker: %s", prereq.DockerCheck(a.engine().Runner()).Message)
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
			a.stopForQuit() // deliberately outside the prompt timeout
		}
		return false
	case <-time.After(quitPromptTimeout):
		return false
	}
}

// askBeforeQuit returns the operator's choice, or "" when there is nothing to ask
// about - no install yet, or nothing running to take down.
func (a *App) askBeforeQuit() string {
	if !a.clinicRunning() {
		return ""
	}
	name := a.loadConfig().MDNSName
	if name == "" {
		name = "care.local"
	}
	sel, err := wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
		Type:  wruntime.QuestionDialog,
		Title: "Quit CARE Desktop?",
		Message: "CARE will keep running in the background, but " + name +
			" will stop working for other devices on the clinic's WiFi until this app is open again.\n\n" +
			"You can also shut the clinic down completely.",
		Buttons:       []string{keepRunning, stopAndQuit},
		DefaultButton: keepRunning, // the destructive one must not be what Enter hits
		CancelButton:  keepRunning,
	})
	if err != nil {
		return "" // no dialog means no answer; never trap the operator in the app
	}
	return sel
}

func (a *App) clinicRunning() bool {
	if !a.loadConfig().SetupDone {
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

// stopForQuit stops the stack on the way out, bounded so a wedged Docker cannot
// hold the app open. `compose stop` also marks the containers as deliberately
// stopped, so their restart:unless-stopped policy will not bring them back at the
// next boot - which is what the operator asked for.
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
