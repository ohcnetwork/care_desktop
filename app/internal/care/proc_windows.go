//go:build windows

package care

import (
	"os/exec"
	"syscall"
)

// hideConsole stops a child console process (docker, git, certutil, powershell)
// from popping a cmd window - without it the app flashes a window on every poll.
func hideConsole(c *exec.Cmd) {
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.SysProcAttr.HideWindow = true
	c.SysProcAttr.CreationFlags |= 0x08000000 // CREATE_NO_WINDOW
}
