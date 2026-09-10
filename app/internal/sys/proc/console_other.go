//go:build !windows

package proc

import "os/exec"

func hideConsole(*exec.Cmd) {}
