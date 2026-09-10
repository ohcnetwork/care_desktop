// Package hostname records and restores the machine name that "rename" mDNS
// mode replaces. See docs/architecture.md#mdns-advertising-and-self-heal.
package hostname

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

// backupPath is the machine name from before a rename-mode install. It lives in the install dir
// because the engine has no other state store (the CLI has no config file), so
// uninstall must read it before it deletes the install dir.
func backupPath(installDir string) string {
	return filepath.Join(installDir, ".previous-hostname")
}

// SavePrevious records the name we are about to replace. It never
// overwrites an existing record: a second install would otherwise save CARE's
// own name over the operator's original and make the rename permanent.
func SavePrevious(installDir, ours, name string) {
	name = strings.TrimSpace(name)
	if name == "" || name == ours {
		return
	}
	if _, err := os.Stat(backupPath(installDir)); err == nil {
		return
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(backupPath(installDir), []byte(name+"\n"), 0o644)
}

func Previous(installDir string) string {
	b, err := os.ReadFile(backupPath(installDir))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// Restore puts the machine name back after a rename-mode install,
// returning a description of what was left if it could not. The rename needs
// sudo, which a windowed app has no terminal for, so the failure path hands the
// operator the exact command rather than pretending it is done.
func Restore(run proc.Runner, log func(string), installDir, ours, previous string) string {
	if previous == "" {
		return "" // never renamed - the default advertise mode touches no names
	}
	name := ours
	var get []string
	var set []string
	switch runtime.GOOS {
	case "darwin":
		get = []string{"scutil", "--get", "LocalHostName"}
		set = []string{"sudo", "scutil", "--set", "LocalHostName", previous}
	case "linux":
		get = []string{"hostnamectl", "--static"}
		set = []string{"sudo", "hostnamectl", "set-hostname", previous}
	default:
		return ""
	}
	// Only undo our own rename: if the name is no longer the one we set, someone
	// chose it deliberately and it is not ours to change back.
	if cur, _ := run.Capture(get[0], get[1:]...); strings.TrimSpace(cur) != name {
		return ""
	}
	logln(log, "Restoring this computer's name to '"+previous+"'...")
	if err := run.Run(set[0], set[1:]...); err != nil {
		return "This computer is still named \"" + name + "\". Restore it with:  " + strings.Join(set, " ")
	}
	return ""
}

func logln(log func(string), s string) {
	if log != nil {
		log(s)
	}
}
