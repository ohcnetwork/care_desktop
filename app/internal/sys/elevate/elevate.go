package elevate

import (
	"runtime"
	"strings"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

type Step struct {
	What string
	Sh   string
	PS   string
}

func PSQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func ShQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func OSAQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

func Run(sh string, elevated bool) error {
	switch {
	case !elevated:
		return proc.Command("sh", "-c", sh).Run()
	case runtime.GOOS == "darwin":
		return proc.Command("osascript", "-e",
			"do shell script "+OSAQuote(sh)+" with administrator privileges").Run()
	default:
		return proc.Command("pkexec", "sh", "-c", sh).Run()
	}
}

func Steps(steps []Step) error {
	if len(steps) == 0 {
		return nil
	}
	if runtime.GOOS == "windows" {
		return proc.Command("powershell", "-NoProfile", "-Command",
			elevatedPS(stepScript(steps, true))).Run()
	}
	return Run(stepScript(steps, false), true)
}

func elevatedPS(inner string) string {
	return "$ErrorActionPreference = 'Stop'; $p = Start-Process powershell -Verb RunAs -WindowStyle Hidden -Wait -PassThru " +
		"-ArgumentList '-NoProfile','-WindowStyle','Hidden','-Command'," + PSQuote(inner) + "; exit $p.ExitCode"
}

func stepScript(steps []Step, windows bool) string {
	pick := func(s Step) string { return s.Sh }
	if windows {
		pick = func(s Step) string { return s.PS }
	}
	parts := make([]string, 0, len(steps))
	for _, s := range steps {
		if windows {
			parts = append(parts, "& { "+pick(s)+" }; if (-not $?) { exit 1 }")
		} else {
			parts = append(parts, "( "+pick(s)+" )")
		}
	}
	if windows {
		return "$ErrorActionPreference = 'Stop'; " + strings.Join(parts, "; ")
	}
	return strings.Join(parts, " && ")
}
