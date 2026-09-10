//go:build windows

package proc

import (
	"os/exec"
	"syscall"
)

// hideConsole stops a child console process from popping a cmd window.
func hideConsole(c *exec.Cmd) {
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.SysProcAttr.HideWindow = true
	c.SysProcAttr.CreationFlags |= 0x08000000 // CREATE_NO_WINDOW
}
