//go:build windows

package proc

import (
	"os/exec"
	"syscall"
)

func hideConsole(c *exec.Cmd) {
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.SysProcAttr.HideWindow = true
	c.SysProcAttr.CreationFlags |= 0x08000000 // CREATE_NO_WINDOW
}
