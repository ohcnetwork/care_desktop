// Package reboot detects whether the machine must restart before Docker will
// work, and performs the restart. See docs/architecture.md.
package reboot

import (
	"fmt"
	"os/user"
	"runtime"
	"strings"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/elevate"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

// Plan says whether this machine must restart before the prerequisites will
// work, and what to put on the button.
type Plan struct {
	Needed bool   `json:"needed"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Label  string `json:"label"`
}

// Registry flags Windows sets when a component needs a restart to finish. There
// is no single one; Microsoft's guidance is to check several.
var windowsRebootKeys = [][]string{
	{`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Component Based Servicing\RebootPending`},
	{`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update\RebootRequired`},
	{`HKLM\SYSTEM\CurrentControlSet\Control\Session Manager`, "/v", "PendingFileRenameOperations"},
}

// Check reports what is outstanding. Consult it only right after installing
// something: a restart Windows wanted for its own reasons is not ours to nag
// about.
func Check() Plan {
	switch runtime.GOOS {
	case "windows":
		if !windowsPending() {
			return Plan{}
		}
		return Plan{
			Needed: true,
			Title:  "Restart to finish setting up Docker",
			Detail: "Windows needs to restart before Docker can run - it has to turn on WSL 2, " +
				"which only takes effect after a restart. CARE Desktop will open again by itself " +
				"and pick up where you left off.",
			Label: "Restart now",
		}
	case "linux":
		if !dockerGroupPending() {
			return Plan{}
		}
		return Plan{
			Needed: true,
			Title:  "Restart to finish setting up Docker",
			Detail: "Your account was added to the \"docker\" group, which only takes effect " +
				"after you sign in again. Restarting is the surest way. CARE Desktop will open " +
				"again by itself and pick up where you left off.",
			Label: "Restart now",
		}
	default:
		// macOS never needs one: Docker Desktop is usable as soon as it starts.
		return Plan{}
	}
}

func windowsPending() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	for _, key := range windowsRebootKeys {
		if proc.Command("reg", append([]string{"query"}, key...)...).Run() == nil {
			return true
		}
	}
	return false
}

// dockerGroupPending reports that the docker group lists this user but the
// running session does not yet carry it: the classic "log out and back in".
func dockerGroupPending() bool {
	if runtime.GOOS != "linux" || !proc.Exists("docker") {
		return false
	}
	u, err := user.Current()
	if err != nil {
		return false
	}
	// `id -nG` is the session's live group set; the group file is the intended
	// one. A difference between them is exactly the pending re-login.
	live, err := proc.Command("id", "-nG").Output()
	if err != nil || containsWord(string(live), "docker") {
		return false
	}
	members, err := proc.Command("getent", "group", "docker").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(members), u.Username)
}

func containsWord(haystack, word string) bool {
	for _, f := range strings.Fields(haystack) {
		if f == word {
			return true
		}
	}
	return false
}

// Now asks the OS to restart gracefully, so open applications get their chance
// to save. None of these need a password: macOS and systemd both let the
// logged-in user restart their own machine.
func Now() error {
	switch runtime.GOOS {
	case "darwin":
		// The AppleScript route is graceful and needs no privileges; `shutdown -r`
		// is a hard kill that needs sudo and can lose work.
		return proc.Command("osascript", "-e", `tell application "System Events" to restart`).Run()
	case "windows":
		// A few seconds' grace so this process can exit cleanly first.
		return proc.Command("shutdown", "/r", "/t", "5", "/c",
			"CARE Desktop is restarting this computer to finish setting up Docker.").Run()
	case "linux":
		if err := proc.Command("systemctl", "reboot").Run(); err == nil {
			return nil
		}
		// No logind session, or polkit said no: ask for the password instead.
		return elevate.Run("systemctl reboot", true)
	}
	return fmt.Errorf("restarting isn't supported on %s", runtime.GOOS)
}
