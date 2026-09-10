// Package proc runs external commands. It is the only place in the app that
// spawns a process. See docs/architecture.md.
package proc

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Runner executes commands with a fixed working directory, environment, and log
// sink. The zero value is usable: it inherits the current dir and environment
// and discards output.
type Runner struct {
	Dir string
	Env []string
	Log func(string)
}

func (r Runner) logln(s string) {
	if r.Log != nil {
		r.Log(s)
	}
}

// Command builds an exec.Cmd with the console window hidden on Windows.
func Command(name string, args ...string) *exec.Cmd {
	c := exec.Command(name, args...)
	hideConsole(c)
	return c
}

func (r Runner) cmd(name string, args ...string) *exec.Cmd {
	c := Command(name, args...)
	c.Dir = r.Dir
	c.Env = r.Env
	return c
}

// Run streams stdout and stderr to the log sink, one line at a time.
func (r Runner) Run(name string, args ...string) error {
	return r.RunWith(nil, name, args...)
}

// RunWith is Run with extra environment entries appended.
func (r Runner) RunWith(extraEnv []string, name string, args ...string) error {
	cmd := r.cmd(name, args...)
	if len(extraEnv) > 0 {
		cmd.Env = append(cmd.Env, extraEnv...)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", name, err)
	}
	var wg sync.WaitGroup
	stream := func(rd io.Reader) {
		defer wg.Done()
		sc := bufio.NewScanner(rd)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			r.logln(sc.Text())
		}
	}
	wg.Add(2)
	go stream(stdout)
	go stream(stderr)
	wg.Wait()
	return cmd.Wait()
}

// Capture returns trimmed stdout without streaming.
func (r Runner) Capture(name string, args ...string) (string, error) {
	out, err := r.cmd(name, args...).Output()
	return strings.TrimSpace(string(out)), err
}

// Lines returns Capture's non-empty output lines, trimmed.
func (r Runner) Lines(name string, args ...string) []string {
	out, err := r.Capture(name, args...)
	if err != nil || out == "" {
		return nil
	}
	var lines []string
	for _, ln := range strings.Split(out, "\n") {
		if s := strings.Trim(ln, " \t\r"); s != "" {
			lines = append(lines, s)
		}
	}
	return lines
}

// Succeeds reports whether the command exits zero, discarding all output.
func (r Runner) Succeeds(name string, args ...string) bool {
	return r.cmd(name, args...).Run() == nil
}

// AugmentedPath prepends the directories where docker and git live. A
// GUI-launched app inherits a minimal PATH that usually omits them.
func AugmentedPath() string {
	var parts []string
	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
		parts = []string{
			`C:\Program Files\Docker\Docker\resources\bin`,
			`C:\Program Files\Git\bin`,
			`C:\Program Files\Git\cmd`,
		}
	} else {
		parts = []string{
			"/opt/homebrew/bin", "/opt/homebrew/sbin",
			"/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin",
		}
	}
	if existing := os.Getenv("PATH"); existing != "" {
		parts = append(parts, existing)
	}
	return strings.Join(parts, sep)
}

// FixPath widens the process PATH so binary lookups succeed. exec.Command
// resolves against the process PATH, not a command's Env, so this must run once
// at startup before anything else spawns a process.
func FixPath() {
	var parts []string
	if sp := loginShellPath(); sp != "" {
		parts = append(parts, sp)
	}
	parts = append(parts, AugmentedPath())
	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
	}
	_ = os.Setenv("PATH", strings.Join(parts, sep))
}

// loginShellPath asks the user's login shell for its PATH, which is where docker
// and git are known to resolve. Unix only, bounded so a slow profile cannot hang
// startup.
func loginShellPath() string {
	if runtime.GOOS == "windows" {
		return ""
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, shell, "-lc", "echo $PATH").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Exists reports whether a program resolves on PATH.
func Exists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// FileExists reports whether p exists.
func FileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
