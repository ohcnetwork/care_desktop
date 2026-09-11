// Package applog writes the application's diagnostic log to a file on disk.
//
// Everything the app streams - every line of docker and git output, every failed
// action, every panic - belongs here as well as in the UI, because the UI keeps
// nothing. Its buffer is 300 lines, it is discarded when the window closes, and in
// the panel it is not rendered at all. When a clinic's install fails, this file is
// the only thing anyone can send to whoever supports them.
//
// Three rules follow from that, and together they are why this exists instead of
// Wails' logger.NewFileLogger:
//
//  1. Logging must never take the app down. Every error here is swallowed. A full
//     disk or a read-only home directory is a reason to lose logs, never a reason
//     to stop serving a clinic - which is what NewFileLogger's log.Fatal does.
//  2. It must be bounded. A retrying image build emits thousands of lines a minute
//     and there is no upstream that will stop it.
//  3. It must be safe to call from anywhere. proc.Runner alone streams stdout and
//     stderr into this sink from two separate goroutines.
//
// The seven single-string methods satisfy Wails' logger.Logger interface without
// importing Wails - Go interfaces are structural, and internal/ must stay free of
// Wails for the engine to remain portable (CI enforces this).
package applog

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// maxLine caps one written line. proc.Runner's scanner allows up to 1MB, and a
// base64 blob in build output should not become a 1MB log line.
const maxLine = 8 << 10

// Logger appends timestamped, redacted lines to a rotating file. The zero value
// and a nil *Logger are both usable and discard everything, so callers never need
// to check before writing.
type Logger struct {
	// OnFatal is called by Fatal instead of exiting, letting the app report the
	// failure the way it reports every other one. Nil means exit.
	OnFatal func(string)

	mu   sync.Mutex
	f    *os.File
	path string
	size int64
}

// Open starts logging under dir, falling back to the platform's convention when
// dir is empty. It never fails: if the directory or file cannot be opened the
// returned Logger simply discards, because losing logs must not stop the app.
func Open(dir string) *Logger {
	l := &Logger{}
	dir = Dir(dir)
	if dir == "" {
		return l
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return l
	}
	path := filepath.Join(dir, logName)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return l
	}
	l.f, l.path = f, path
	if st, err := f.Stat(); err == nil {
		l.size = st.Size()
	}
	l.Write(sessionRule())
	return l
}

// Write appends one line: timestamp, then the message with secrets scrubbed.
func (l *Logger) Write(line string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return
	}
	line = Redact(strings.TrimRight(line, "\r\n"))
	if len(line) > maxLine {
		line = line[:maxLine] + " ...[truncated]"
	}
	// Local time, not UTC: this log is read next to an operator saying "it broke
	// around two o'clock".
	b := []byte(time.Now().Format("2006-01-02T15:04:05.000Z07:00") + "  " + line + "\n")
	l.rotateIfFull(len(b))
	n, err := l.f.Write(b)
	if err != nil {
		return // nothing useful to do; never propagate
	}
	l.size += int64(n)
}

// Writef is Write with formatting.
func (l *Logger) Writef(format string, args ...any) { l.Write(fmt.Sprintf(format, args...)) }

// Path is the file being written, or "" when logging is disabled.
func (l *Logger) Path() string {
	if l == nil {
		return ""
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.path
}

// Folder is the directory holding the log files, for "Open Logs Folder".
func (l *Logger) Folder() string {
	if p := l.Path(); p != "" {
		return filepath.Dir(p)
	}
	return ""
}

// Close flushes and releases the file. Safe to call more than once.
func (l *Logger) Close() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return
	}
	_ = l.f.Close()
	l.f = nil
}

// Header records what a supporter needs before reading anything else: which build
// this is, on what, and against which install. Written once per launch.
//
// Deliberately no Docker version: probing for it means spawning a process, and
// this runs on the path to the first window. The app logs it from startup instead.
func (l *Logger) Header(version, installDir, clinicName string) {
	l.Writef("CARE Desktop %s · %s/%s · Go %s", version, runtime.GOOS, runtime.GOARCH, runtime.Version())
	l.Writef("install dir: %s", fallback(installDir, "(none yet)"))
	l.Writef("clinic name: %s", fallback(clinicName, "(unset)"))
	l.Writef("log file: %s", l.Path())
}

func fallback(s, alt string) string {
	if strings.TrimSpace(s) == "" {
		return alt
	}
	return s
}

func sessionRule() string {
	return "──────── session start ────────"
}

// --- Wails logger.Logger ------------------------------------------------------
//
// Wails' own diagnostics (asset server, bindings, IPC) reach the file through
// these. They are tagged so they read apart from the app's own lines, which are
// the ones an operator's problem usually lives in.

const wailsTag = "wails "

func (l *Logger) Print(m string)   { l.Write(wailsTag + m) }
func (l *Logger) Trace(m string)   { l.Write(wailsTag + "TRACE " + m) }
func (l *Logger) Debug(m string)   { l.Write(wailsTag + "DEBUG " + m) }
func (l *Logger) Info(m string)    { l.Write(wailsTag + "INFO  " + m) }
func (l *Logger) Warning(m string) { l.Write(wailsTag + "WARN  " + m) }
func (l *Logger) Error(m string)   { l.Write(wailsTag + "ERROR " + m) }

// Fatal hands off to OnFatal so the app can show the operator a dialog. Wails'
// own FileLogger calls os.Exit here, which in a packaged build means the window
// disappears with nothing said.
func (l *Logger) Fatal(m string) {
	l.Write(wailsTag + "FATAL " + m)
	if l != nil && l.OnFatal != nil {
		l.OnFatal(m)
		return
	}
	l.Close()
	os.Exit(1)
}
