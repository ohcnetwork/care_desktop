package care

import (
	"runtime"
	"strings"
)

// privilegedStep is one administrator-requiring action, per platform.
type privilegedStep struct {
	what string // shown in the confirmation
	sh   string // POSIX (macOS, Linux)
	ps   string // PowerShell (Windows)
}

// ensureLocalAccess makes https://<name>.local work in this machine's own browser.
// Both steps need admin and both need Caddy up, so they share ONE approval and one
// elevation. See docs/architecture.md#one-install-one-approval.
func (e *Engine) ensureLocalAccess() {
	host := e.host()

	var steps []privilegedStep
	if step, need := e.hostsStep(host); need {
		steps = append(steps, step)
	}
	step, cleanup, need := e.caStep(host)
	defer cleanup()
	if need {
		steps = append(steps, step)
	}
	if len(steps) == 0 {
		return
	}

	title, message := confirmPrompt(host, steps)
	if e.Confirm == nil || !e.Confirm(title, message) {
		e.logln("Skipped - other devices can still use the clinic, but this computer's own " +
			"browser may not open https://" + host + "/. Starting CARE again will offer this once more.")
		return
	}
	if err := e.runPrivilegedSteps(steps); err != nil {
		e.logln("Could not finish local setup (" + err.Error() +
			"). Other devices are unaffected; open http://" + host + "/setup to do it by hand.")
		return
	}
	e.logln("This computer can now open https://" + host + "/.")
}

func confirmPrompt(host string, steps []privilegedStep) (title, message string) {
	var what strings.Builder
	for _, s := range steps {
		what.WriteString("  •  " + s.what + "\n")
	}
	return "Finish setting up " + host + " on this computer?",
		"To open https://" + host + " in this computer's own browser, CARE needs to:\n\n" +
			what.String() +
			"\nThis asks for your administrator password once. Other devices are unaffected."
}

func (e *Engine) runPrivilegedSteps(steps []privilegedStep) error {
	pick := func(s privilegedStep) string { return s.sh }
	if runtime.GOOS == "windows" {
		pick = func(s privilegedStep) string { return s.ps }
	}
	parts := make([]string, 0, len(steps))
	for _, s := range steps {
		parts = append(parts, pick(s))
	}
	// "; " not "&&": a failing first step must not skip the second.
	joined := strings.Join(parts, "; ")

	if runtime.GOOS == "windows" {
		ps := "Start-Process powershell -Verb RunAs -Wait -ArgumentList '-NoProfile','-Command'," +
			psSingleQuote(joined)
		return newCmd("powershell", "-NoProfile", "-Command", ps).Run()
	}
	return e.runPrivileged(joined, true)
}
