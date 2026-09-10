package main

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ohcnetwork/care_desktop/app/internal/backup"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// --- restore ----------------------------------------------------------------

// ListBackups returns the restorable points in the backup folder (newest first)
// for the panel's restore dropdown.
func (a *App) ListBackups() ([]backup.Backup, error) {
	if _, err := os.Stat(filepath.Join(a.installDir(), "docker-compose.yml")); err != nil {
		return nil, nil // not set up yet - no backups to offer
	}
	return a.engine().Backups().ListBackups()
}

// RestoreBackup restores async. For an encrypted backup, "" passphrase falls back to
// the keychain; remember saves the one that worked.
func (a *App) RestoreBackup(dbDump, filesArchive, passphrase string, remember bool) error {
	if _, err := os.Stat(filepath.Join(a.installDir(), "docker-compose.yml")); err != nil {
		return errors.New("not set up yet - run the first-time setup")
	}
	if passphrase == "" {
		passphrase = backup.LoadPassword()
	}
	if passphrase != "" && remember {
		_ = backup.StorePassword(passphrase)
	}
	e := a.engine()
	a.run(func() error { return e.Backups().Restore(dbDump, filesArchive, passphrase) }, false, "restore")
	return nil
}

// --- where the backups go ---------------------------------------------------

// GetBackupDir is the folder backups are written to right now.
func (a *App) GetBackupDir() string { return a.engine().BackupDirPath() }

// SetBackupDir moves future backups to a folder under dir (typically a USB
// drive). Existing backups are deliberately left where they are: they can be
// gigabytes, the old drive may be the one being retired, and silently copying
// patient data between drives is not something to do without being asked.
//
// The recovery key is copied across, because a backup folder has to be
// self-contained to be restorable on another machine - that is the whole reason
// backup-key.pem.enc sits beside the dumps.
//
// Returns the new folder so the UI can show it.
func (a *App) SetBackupDir(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", errors.New("choose a folder for the backups")
	}
	if problem := a.ValidateBackupDir(dir); problem != "" {
		return "", errors.New(problem)
	}
	target := filepath.Join(dir, "care-db-backups")
	if err := os.MkdirAll(target, 0o755); err != nil {
		return "", err
	}

	cfg := a.loadConfig()
	previous := a.engine().BackupDirPath()
	cfg.BackupDir = target
	if err := a.saveConfig(cfg); err != nil {
		return "", err
	}

	e := a.engine() // rebuilt so BACKUP_DIR points at the new folder
	if err := e.Backups().CopyRecoveryKey(); err != nil {
		a.logln("note: couldn't copy the recovery key into the new folder (" + err.Error() +
			"). Restores on THIS computer still work; the folder is not yet self-contained.")
	}

	// The sidecar's /backups mount is fixed when the container is created, so it
	// keeps writing to the old folder until it is recreated.
	a.logln("Backups will now go to " + target)
	if previous != "" && previous != target {
		a.logln("Earlier backups were left in " + previous + " - move them yourself if you want them together.")
	}
	if err := e.RestartBackupSidecar(); err != nil {
		return target, errors.New("the folder was saved, but the backup service didn't pick it up: " + err.Error())
	}
	return target, nil
}

// --- restoring a backup from somewhere else ---------------------------------

// importable matches the dump filenames a backup folder contains, so the picker's
// result is checked before it is used for anything.
var importable = regexp.MustCompile(`^care-(?:manual-)?(\d{8}-\d{6})\.dump(?:\.enc)?$`)

// ImportedBackup describes a dump the operator picked off a USB stick, so the UI
// can show what it found before anything destructive is offered.
type ImportedBackup struct {
	Path         string `json:"path"`          // the chosen dump
	Dir          string `json:"dir"`           // folder it lives in
	DBDump       string `json:"db_dump"`       // its filename
	FilesArchive string `json:"files_archive"` // matching files-<ts> beside it, or ""
	Label        string `json:"label"`
	Encrypted    bool   `json:"encrypted"`
	HasKey       bool   `json:"has_key"` // backup-key.pem.enc sits beside it
}

// InspectBackupFile validates a picked file and reports what restoring it would
// involve, without touching anything. The UI calls this first so the operator
// sees whether the files archive and the recovery key came along before deciding.
func (a *App) InspectBackupFile(path string) (ImportedBackup, error) {
	var out ImportedBackup
	path = strings.TrimSpace(path)
	if path == "" {
		return out, errors.New("no file chosen")
	}
	name := filepath.Base(path)
	m := importable.FindStringSubmatch(name)
	if m == nil {
		return out, errors.New("that isn't a CARE database backup. Choose a file named like " +
			"care-20260101-020000.dump.enc - the one from the clinic's backup folder.")
	}
	if _, err := os.Stat(path); err != nil {
		return out, errors.New("couldn't open that file: " + err.Error())
	}

	dir := filepath.Dir(path)
	out = ImportedBackup{
		Path:      path,
		Dir:       dir,
		DBDump:    name,
		Encrypted: strings.HasSuffix(name, ".enc"),
	}
	// The matching files archive is the same timestamp, either sealed or not.
	for _, candidate := range []string{"files-" + m[1] + ".tar.gz.enc", "files-" + m[1] + ".tar.gz"} {
		if _, err := os.Stat(filepath.Join(dir, candidate)); err == nil {
			out.FilesArchive = candidate
			break
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "backup-key.pem.enc")); err == nil {
		out.HasKey = true
	}

	scope := "database only"
	if out.FilesArchive != "" {
		scope = "database + files"
	}
	out.Label = m[1] + " - " + scope
	if out.Encrypted {
		out.Label += " - encrypted"
	}
	return out, nil
}

// RestoreFromFile restores a backup that lives outside this install's backup
// folder. The folder is mounted read-only for the restore, so a backup carried
// from another computer is opened with ITS OWN recovery key rather than this
// machine's - which is what makes moving a clinic work at all.
func (a *App) RestoreFromFile(path, passphrase string, remember bool) error {
	if _, err := os.Stat(filepath.Join(a.installDir(), "docker-compose.yml")); err != nil {
		return errors.New("not set up yet - run the first-time setup")
	}
	found, err := a.InspectBackupFile(path)
	if err != nil {
		return err
	}
	if found.Encrypted && !found.HasKey {
		// The local key belongs to a different keypair, so it cannot open this.
		// Saying so now beats failing after the services have been stopped.
		if passphrase == "" {
			passphrase = backup.LoadPassword()
		}
		a.logln("note: no backup-key.pem.enc beside that file - trying this computer's own key, " +
			"which only works if the backup came from this clinic.")
	}
	if passphrase == "" {
		passphrase = backup.LoadPassword()
	}
	if passphrase != "" && remember {
		_ = backup.StorePassword(passphrase)
	}
	e := a.engine()
	a.run(func() error {
		return e.Backups().RestoreFrom(found.Dir, found.DBDump, found.FilesArchive, passphrase)
	}, false, "restore")
	return nil
}

// ChooseBackupFile opens the system file picker at the current backup folder and
// returns the chosen path ("" if cancelled).
func (a *App) ChooseBackupFile() string {
	opts := wruntime.OpenDialogOptions{
		Title:            "Choose a backup file",
		DefaultDirectory: a.engine().BackupDirPath(),
		Filters: []wruntime.FileFilter{
			{DisplayName: "CARE backups (care-*.dump, care-*.dump.enc)", Pattern: "*.dump;*.enc"},
			{DisplayName: "All files", Pattern: "*"},
		},
	}
	path, err := wruntime.OpenFileDialog(a.ctx, opts)
	if err != nil {
		return ""
	}
	return path
}
