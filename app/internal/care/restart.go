package care

// Finishing an install sometimes needs the machine restarted - turning on WSL 2
// for Docker Desktop is the big one. Telling a clinic operator to "restart when
// convenient" strands the wizard half-done, so the app detects it, offers it,
// and does it.

import (
	"fmt"
	"os/user"
	"runtime"
	"strings"
)

// RestartPlan says whether this machine must restart before the prerequisites
// will work, and what to put on the button.
type RestartPlan struct {
	Needed bool   `json:"needed"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Label  string `json:"label"`
}

// Registry flags Windows sets when a component needs a restart to finish. There
// is no single one - Microsoft's own guidance is to check several.
var windowsRebootKeys = [][]string{
	{`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Component Based Servicing\RebootPending`},
	{`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update\RebootRequired`},
	{`HKLM\SYSTEM\CurrentControlSet\Control\Session Manager`, "/v", "PendingFileRenameOperations"},
}

// RestartPlan reports what is outstanding. It is deliberately only consulted
// right after the app installs something: a restart Windows wanted for its own
// reasons is not ours to nag about.
func (e *Engine) RestartPlan() RestartPlan {
	switch runtime.GOOS {
	case "windows":
		if !windowsRebootPending() {
			return RestartPlan{}
		}
		return RestartPlan{
			Needed: true,
			Title:  "Restart to finish setting up Docker",
			Detail: "Windows needs to restart before Docker can run - it has to turn on WSL 2, " +
				"which only takes effect after a restart. CARE Desktop will open again by itself " +
				"and pick up where you left off.",
			Label: "Restart now",
		}
	case "linux":
		if !dockerGroupPending() {
			return RestartPlan{}
		}
		return RestartPlan{
			Needed: true,
			Title:  "Restart to finish setting up Docker",
			Detail: "Your account was added to the \"docker\" group, which only takes effect " +
				"after you sign in again. Restarting is the surest way. CARE Desktop will open " +
				"again by itself and pick up where you left off.",
			Label: "Restart now",
		}
	default:
		// macOS never needs one: Docker Desktop is usable as soon as it starts.
		return RestartPlan{}
	}
}

func windowsRebootPending() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	for _, key := range windowsRebootKeys {
		if newCmd("reg", append([]string{"query"}, key...)...).Run() == nil {
			return true
		}
	}
	return false
}

// dockerGroupPending reports that the docker group lists this user but the
// running session does not yet carry it - the classic "log out and back in".
func dockerGroupPending() bool {
	if runtime.GOOS != "linux" || !hasCommand("docker") {
		return false
	}
	u, err := user.Current()
	if err != nil {
		return false
	}
	// `id -nG` is the session's live group set; the group file is the intended
	// one. A difference between them is exactly the pending re-login.
	live, err := newCmd("id", "-nG").Output()
	if err != nil || containsWord(string(live), "docker") {
		return false
	}
	members, err := newCmd("getent", "group", "docker").Output()
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

// RestartComputer asks the OS to restart, gracefully, so open applications get their
// chance to save. None of these need a password: macOS and systemd both let the
// logged-in user restart their own machine.
func (e *Engine) RestartComputer() error {
	e.logln("Restarting this computer...")
	switch runtime.GOOS {
	case "darwin":
		// The AppleScript route is the graceful one and needs no privileges;
		// `shutdown -r` is a hard kill that needs sudo and can lose work.
		return newCmd("osascript", "-e", `tell application "System Events" to restart`).Run()
	case "windows":
		// A few seconds' grace so this process can exit cleanly first.
		return newCmd("shutdown", "/r", "/t", "5", "/c",
			"CARE Desktop is restarting this computer to finish setting up Docker.").Run()
	case "linux":
		if err := newCmd("systemctl", "reboot").Run(); err == nil {
			return nil
		}
		// No logind session, or polkit said no - ask for the password instead.
		return e.runPrivileged("systemctl reboot", true)
	}
	return fmt.Errorf("restarting isn't supported on %s", runtime.GOOS)
}
