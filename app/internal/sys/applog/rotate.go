package applog

import (
	"os"
	"path/filepath"
	"strconv"
)

// maxSize is when the current file is rolled aside, and maxFiles how many are
// kept in total (current included) - a hard ceiling of maxSize*maxFiles on disk.
// A clinic's machine must never fill up because a build kept retrying.
//
// Variables rather than constants so the rotation test can shrink them; nothing
// in the app reassigns them.
var (
	maxSize  int64 = 5 << 20
	maxFiles       = 3
)

// rotateIfFull rolls the log when the next write would take it past maxSize.
// Called with the lock held. Any failure leaves the current file in place and
// logging continues: an unrotated log is better than a lost one.
func (l *Logger) rotateIfFull(next int) {
	if l.f == nil || l.size+int64(next) <= maxSize {
		return
	}
	dir := filepath.Dir(l.path)
	_ = l.f.Close()
	l.f = nil

	// Drop the oldest, then shift each survivor down: care.1.log -> care.2.log.
	_ = os.Remove(rolled(dir, maxFiles-1))
	for i := maxFiles - 2; i >= 1; i-- {
		_ = os.Rename(rolled(dir, i), rolled(dir, i+1))
	}
	if err := os.Rename(l.path, rolled(dir, 1)); err != nil {
		// Could not roll it aside - reopen and keep appending rather than stop.
		l.reopen(os.O_APPEND)
		return
	}
	l.reopen(os.O_TRUNC)
}

func (l *Logger) reopen(mode int) {
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|mode, 0o644)
	if err != nil {
		return // discard from here on; never fail the caller
	}
	l.f = f
	l.size = 0
	if st, err := f.Stat(); err == nil {
		l.size = st.Size()
	}
}

// rolled names the nth archived log: care.1.log, care.2.log, ...
func rolled(dir string, n int) string {
	return filepath.Join(dir, "care."+strconv.Itoa(n)+".log")
}
