package clinic

import (
	"strings"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/hosts"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/trust"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/elevate"
)

// ensureLocalAccess makes https://<name>.local work in this machine's own browser.
// Both steps need admin and both need Caddy up, so they share ONE approval and one
// elevation. See docs/architecture.md#one-install-one-approval.
func (e *Clinic) ensureLocalAccess() {
	host := e.host()

	var steps []elevate.Step
	if step, need := hosts.Step(e.Log, host); need {
		steps = append(steps, step)
	}
	step, cleanup, need := trust.Step(e.Log, host, e.caddyRootPEM())
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
	if err := elevate.Steps(steps); err != nil {
		e.logln("Could not finish local setup (" + err.Error() +
			"). Other devices are unaffected; open http://" + host + "/setup to do it by hand.")
		return
	}
	e.logln("This computer can now open https://" + host + "/.")
}

func confirmPrompt(host string, steps []elevate.Step) (title, message string) {
	var what strings.Builder
	for _, s := range steps {
		what.WriteString("  •  " + s.What + "\n")
	}
	return "Finish setting up " + host + " on this computer?",
		"To open https://" + host + " in this computer's own browser, CARE needs to:\n\n" +
			what.String() +
			"\nThis asks for your administrator password once. Other devices are unaffected."
}
