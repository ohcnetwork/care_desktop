package applog

import (
	"os"
	"path/filepath"
	"runtime"
)

const logName = "care.log"

// appFolder is the human-facing name, used where the path is one a person reads.
const appFolder = "CARE Desktop"

// Dir resolves where the log lives. The order mirrors installDir(): an environment
// override for support and debugging, then whatever the operator has configured,
// then the platform's convention.
//
// It is deliberately not asked for during setup. The backup folder is configurable
// because backups are large, grow forever and belong to the operator; the log is
// capped at a few megabytes and its whole job is being findable by someone helping
// from a distance - which is exactly what a per-install path destroys.
func Dir(configured string) string {
	if d := os.Getenv("CARE_DESKTOP_LOG_DIR"); d != "" {
		return d
	}
	if configured != "" {
		return configured
	}
	return defaultDir()
}

// defaultDir is each platform's documented home for application logs. Go's stdlib
// has no helper for this: UserConfigDir is Application Support on macOS and
// UserCacheDir is Caches, and logs belong in neither.
func defaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	switch runtime.GOOS {
	case "darwin":
		// Apple's location, and Console.app lists anything here under Log Reports -
		// so an operator can be walked to it without being given a path at all.
		return filepath.Join(home, "Library", "Logs", appFolder)
	case "windows":
		// Local rather than Roaming: logs are specific to this machine and must not
		// be synced into a roaming domain profile.
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, appFolder, "Logs")
	default:
		// XDG grew a "state" directory precisely for logs and history - not share,
		// which is user data, and not cache, which may be deleted at any time.
		base := os.Getenv("XDG_STATE_HOME")
		if base == "" {
			base = filepath.Join(home, ".local", "state")
		}
		return filepath.Join(base, "care-desktop")
	}
}
