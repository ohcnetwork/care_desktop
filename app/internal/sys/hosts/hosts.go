package hosts

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/elevate"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

const marker = "# care-desktop"

func path() string {
	if runtime.GOOS != "windows" {
		return "/etc/hosts"
	}
	win := os.Getenv("WINDIR")
	if win == "" {
		win = `C:\Windows`
	}
	return filepath.Join(win, "System32", "drivers", "etc", "hosts")
}

func hasEntry(data, host string) bool {
	for _, ln := range strings.Split(data, "\n") {
		line := strings.TrimSpace(ln)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i] // drop any trailing comment
		}
		for _, field := range strings.Fields(line) {
			if strings.EqualFold(field, host) {
				return true
			}
		}
	}
	return false
}

func line(host string) string { return "127.0.0.1 " + host + " " + marker }

func addSh(host string) string {
	p := path()
	return "echo '' >> " + p + "; echo " + elevate.ShQuote(line(host)) + " >> " + p
}

func addPS(host string) string {
	return `Add-Content -LiteralPath "$env:WINDIR\System32\drivers\etc\hosts" -Value ` +
		elevate.PSQuote(line(host))
}

func Step(log func(string), host string) (elevate.Step, bool) {
	if data, err := os.ReadFile(path()); err == nil && hasEntry(string(data), host) {
		return elevate.Step{}, false
	}
	if err := addUnprivileged(host); err == nil {
		logln(log, "Added a hosts entry so https://"+host+"/ opens on this computer.")
		return elevate.Step{}, false
	}
	return elevate.Step{
		What: "add " + host + " to this computer's hosts file, so the name works here",
		Sh:   addSh(host),
		PS:   addPS(host),
	}, true
}

func addUnprivileged(host string) error {
	if runtime.GOOS == "windows" {
		return proc.Command("powershell", "-NoProfile", "-Command", addPS(host)).Run()
	}
	return proc.Command("sh", "-c", addSh(host)).Run()
}

func Remove(log func(string), confirm func(string, string) bool, host string) string {
	data, err := os.ReadFile(path())
	if err != nil || !strings.Contains(string(data), marker) {
		return ""
	}
	logln(log, "Removing the "+host+" hosts entry...")
	if runtime.GOOS == "windows" {
		inner := `$p="$env:WINDIR\System32\drivers\etc\hosts"; (Get-Content -LiteralPath $p) | ` +
			`Where-Object { $_ -notmatch '` + marker + `' } | Set-Content -LiteralPath $p`
		ps := "Start-Process powershell -Verb RunAs -Wait -ArgumentList '-NoProfile','-Command'," +
			elevate.PSQuote(inner)
		_ = proc.Command("powershell", "-NoProfile", "-Command", ps).Run()
		return Leftover(host)
	}
	p := path()
	sh := `t=$(mktemp) && grep -v ` + elevate.ShQuote(marker) + ` ` + p +
		` > "$t" && cat "$t" > ` + p + `; rm -f "$t"`
	if elevate.Run(sh, false) == nil {
		return Leftover(host)
	}
	if confirm == nil || confirm("Remove the "+host+" hosts entry?",
		"Remove the line CARE added to this computer's hosts file?\n\nThis needs administrator approval.") {
		_ = elevate.Run(sh, true)
	}
	return Leftover(host)
}

func Leftover(host string) string {
	data, err := os.ReadFile(path())
	if err != nil || !strings.Contains(string(data), marker) {
		return ""
	}
	return "The line \"" + line(host) + "\" is still in " + path() +
		". Until it is removed this computer resolves " + host + " to itself."
}

func logln(log func(string), s string) {
	if log != nil {
		log(s)
	}
}

func Present() bool {
	data, err := os.ReadFile(path())
	return err == nil && strings.Contains(string(data), marker)
}
