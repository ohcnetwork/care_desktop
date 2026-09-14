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

func (a *App) ListBackups() ([]backup.Backup, error) {
	if _, err := os.Stat(filepath.Join(a.installDir(), "docker-compose.yml")); err != nil {
		return nil, nil // not set up yet - no backups to offer
	}
	return a.engine().Backups().ListBackups()
}

func (a *App) RestoreBackup(dbDump, filesArchive, passphrase string, remember bool) error {
	if _, err := os.Stat(filepath.Join(a.installDir(), "docker-compose.yml")); err != nil {
		return errors.New("not set up yet - run the first-time setup")
	}
	if passphrase == "" {
		passphrase = backup.LoadPassword()
	}
	e := a.engine()
	return a.run(func() error {
		if err := e.Backups().Restore(dbDump, filesArchive, passphrase); err != nil {
			return err
		}
		rememberPassword(passphrase, remember)
		return nil
	}, false, "restore")
}

func rememberPassword(passphrase string, remember bool) {
	if passphrase != "" && remember {
		_ = backup.StorePassword(passphrase)
	}
}

func (a *App) GetBackupDir() string { return a.engine().BackupDirPath() }

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

	e := a.engine()
	if err := e.Backups().CopyRecoveryKey(); err != nil {
		a.logln("note: couldn't copy the recovery key into the new folder (" + err.Error() +
			"). Restores on THIS computer still work; the folder is not yet self-contained.")
	}

	a.logln("Backups will now go to " + target)
	if previous != "" && previous != target {
		a.logln("Earlier backups were left in " + previous + " - move them yourself if you want them together.")
	}
	if err := e.RestartBackupSidecar(); err != nil {
		return target, errors.New("the folder was saved, but the backup service didn't pick it up: " + err.Error())
	}
	return target, nil
}

var importable = regexp.MustCompile(`^care-(?:manual-)?(\d{8}-\d{6})\.dump(?:\.enc)?$`)

type ImportedBackup struct {
	Path         string `json:"path"`
	Dir          string `json:"dir"`
	DBDump       string `json:"db_dump"`
	FilesArchive string `json:"files_archive"`
	Label        string `json:"label"`
	Encrypted    bool   `json:"encrypted"`
	HasKey       bool   `json:"has_key"`
}

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

func (a *App) RestoreFromFile(path, passphrase string, remember bool) error {
	if _, err := os.Stat(filepath.Join(a.installDir(), "docker-compose.yml")); err != nil {
		return errors.New("not set up yet - run the first-time setup")
	}
	found, err := a.InspectBackupFile(path)
	if err != nil {
		return err
	}
	if found.Encrypted && !found.HasKey {
		a.logln("note: no backup-key.pem.enc beside that file - trying this computer's own key, " +
			"which only works if the backup came from this clinic.")
	}
	if passphrase == "" {
		passphrase = backup.LoadPassword()
	}
	e := a.engine()
	return a.run(func() error {
		if err := e.Backups().RestoreFrom(found.Dir, found.DBDump, found.FilesArchive, passphrase); err != nil {
			return err
		}
		rememberPassword(passphrase, remember)
		return nil
	}, false, "restore")
}

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
