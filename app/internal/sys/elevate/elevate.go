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
	pick := func(s Step) string { return s.Sh }
	if runtime.GOOS == "windows" {
		pick = func(s Step) string { return s.PS }
	}
	parts := make([]string, 0, len(steps))
	for _, s := range steps {
		parts = append(parts, pick(s))
	}
	joined := strings.Join(parts, "; ")

	if runtime.GOOS == "windows" {
		ps := "Start-Process powershell -Verb RunAs -Wait -ArgumentList '-NoProfile','-Command'," +
			PSQuote(joined)
		return proc.Command("powershell", "-NoProfile", "-Command", ps).Run()
	}
	return Run(joined, true)
}
