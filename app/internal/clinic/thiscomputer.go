package clinic

import (
	"strings"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/elevate"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/hosts"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/trust"
)

func (e *Clinic) setUpThisComputer() {
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
	var err error
	if len(steps) > 0 {
		title, message := confirmPrompt(host, steps)
		if e.Confirm == nil || !e.Confirm(title, message) {
			e.logln("Skipped - other devices can still use the clinic, but this computer's own " +
				"browser may not open https://" + host + "/. Starting CARE again will offer this once more.")
			return
		}
		err = elevate.Steps(steps)
	}
	e.logln(localSetupResult(host, hosts.HasEntry(host), trust.HostTrusts(host), err))
}

func localSetupResult(host string, hostsReady, trustReady bool, err error) string {
	var incomplete []string
	if !hostsReady {
		incomplete = append(incomplete, "the local hosts entry")
	}
	if !trustReady {
		incomplete = append(incomplete, "certificate trust")
	}
	if len(incomplete) == 0 {
		return "This computer can now open https://" + host + "/."
	}
	detail := ""
	if err != nil {
		detail = " (" + err.Error() + ")"
	}
	return "Could not confirm " + strings.Join(incomplete, " and ") + detail +
		". Other devices are unaffected; try starting CARE again to retry local setup, or ask your administrator for help."
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
