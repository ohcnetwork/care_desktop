package hosts

import (
	"fmt"
	"net"
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
	found := false
	for _, ln := range strings.Split(data, "\n") {
		line := strings.TrimSpace(ln)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i] // drop any trailing comment
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		for _, field := range fields[1:] {
			if strings.EqualFold(field, host) {
				if !net.ParseIP(fields[0]).IsLoopback() {
					return false
				}
				found = true
			}
		}
	}
	return found
}

func line(host string) string { return "127.0.0.1 " + host + " " + marker }

func addSh(host string) string {
	p := elevate.ShQuote(path())
	return "printf '\\n%s\\n' " + elevate.ShQuote(line(host)) + " >> " + p
}

func addPS(host string) string {
	return `Add-Content -LiteralPath "$env:WINDIR\System32\drivers\etc\hosts" -Value ` +
		elevate.PSQuote(line(host))
}

func Step(log func(string), host string) (elevate.Step, bool) {
	if HasEntry(host) {
		return elevate.Step{}, false
	}
	_ = addUnprivileged(host)
	if HasEntry(host) {
		logln(log, "Added a hosts entry so https://"+host+"/ opens on this computer.")
		return elevate.Step{}, false
	}
	return elevate.Step{
		What: "add " + host + " to this computer's hosts file, so the name works here",
		Sh:   addSh(host),
		PS:   addPS(host),
	}, true
}

func HasEntry(host string) bool {
	data, err := os.ReadFile(path())
	return err == nil && hasEntry(string(data), host)
}

func addUnprivileged(host string) error {
	if runtime.GOOS == "windows" {
		return proc.Command("powershell", "-NoProfile", "-Command", addPS(host)).Run()
	}
	return proc.Command("sh", "-c", addSh(host)).Run()
}

func Remove(log func(string), confirm func(string, string) bool, host string) string {
	data, err := os.ReadFile(path())
	if os.IsNotExist(err) || err == nil && !strings.Contains(string(data), marker) {
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
	return removeUnix(confirm, host, path(), elevate.Run)
}

func removeSh(p string) string {
	return `t=$(mktemp) || exit $?; trap 'rm -f "$t"' EXIT; grep -F -v ` +
		elevate.ShQuote(marker) + ` ` + elevate.ShQuote(p) +
		` > "$t"; status=$?; [ "$status" -le 1 ] || exit "$status"; cat "$t" > ` + elevate.ShQuote(p)
}

func removeUnix(confirm func(string, string) bool, host, p string, run func(string, bool) error) string {
	sh := removeSh(p)
	_ = run(sh, false)
	if leftover(host, p) == "" {
		return ""
	}
	if confirm == nil || confirm("Remove the "+host+" hosts entry?",
		"Remove the line CARE added to this computer's hosts file?\n\nThis needs administrator approval.") {
		_ = run(sh, true)
	}
	return leftover(host, p)
}

func Leftover(host string) string {
	return leftover(host, path())
}

func leftover(host, p string) string {
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		return "Could not check CARE's hosts entry in " + p + ": " + err.Error()
	}
	if !strings.Contains(string(data), marker) {
		return ""
	}
	return "The line \"" + line(host) + "\" is still in " + p +
		". Until it is removed this computer resolves " + host + " to itself."
}

func logln(log func(string), s string) {
	if log != nil {
		log(s)
	}
}

func Present() bool {
	present, err := Inspect()
	return present || err != nil
}

func Inspect() (bool, error) {
	return inspect(path())
}

func inspect(p string) (bool, error) {
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("could not inspect %s: %w", p, err)
	}
	return strings.Contains(string(data), marker), nil
}
