//go:build !windows

package care

import "os/exec"

// hideConsole is a no-op off Windows - only Windows spawns console windows.
func hideConsole(*exec.Cmd) {}
