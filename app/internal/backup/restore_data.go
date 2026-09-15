package backup

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type restoreDatabase struct {
	OID string `json:"oid"`
	Tag string `json:"tag"`
}

type restoreDatabases struct {
	Live   *restoreDatabase `json:"live"`
	Staged *restoreDatabase `json:"staged"`
	Old    *restoreDatabase `json:"old"`
}

func (s *Store) startRestoreDB() error {
	if err := s.dc("up", "-d", "--no-recreate", "--wait", "--wait-timeout", "300", "db"); err != nil {
		return fmt.Errorf("cannot start the database without recreating it: %w", err)
	}
	if err := s.dc("exec", "-T", "db", "sh", "-c",
		`pg_isready -q -h 127.0.0.1 -p "${POSTGRES_PORT:-5432}" -U "${POSTGRES_USER:-postgres}" -d postgres`); err != nil {
		return fmt.Errorf("the database is not ready: %w", err)
	}
	return nil
}

func (s *Store) restoreSQLCommand(j *restoreJournal, step, script string, extra ...string) ([]string, []string, error) {
	settings, err := s.restoreSettings()
	if err != nil {
		return nil, nil, err
	}
	if settings["POSTGRES_DB"] != j.Database || settings["POSTGRES_HOST"] != j.Host ||
		settings["POSTGRES_PORT"] != j.Port || settings["POSTGRES_USER"] != j.User {
		return nil, nil, fmt.Errorf("database settings changed since restore %s began; restore the original connection settings before recovery", j.ID)
	}
	env := []string{
		"PGHOST=" + j.Host, "PGPORT=" + j.Port, "PGUSER=" + j.User,
		"PGPASSWORD=" + settings["POSTGRES_PASSWORD"], "PGDATABASE=postgres", "PGCONNECT_TIMEOUT=10",
		"PGAPPNAME=" + j.tag(), "RESTORE_DB=" + j.Database, "RESTORE_STAGE=" + j.stagedDatabase(),
		"RESTORE_OLD=" + j.oldDatabase(), "RESTORE_TAG=" + j.tag(),
		"RESTORE_ORIGINAL_OID=" + j.OriginalOID, "RESTORE_STAGED_OID=" + j.StagedOID,
	}
	args := s.restoreHelperArgs(j, step, j.Project, extra...)
	for _, item := range env {
		name, _, _ := strings.Cut(item, "=")
		args = append(args, "-e", name)
	}
	args = append(args, s.Image, "sh", "-c", script)
	return args, env, nil
}

func (s *Store) runRestoreSQL(j *restoreJournal, step, script string, extra ...string) error {
	args, env, err := s.restoreSQLCommand(j, step, script, extra...)
	if err != nil {
		return err
	}
	return s.run.RunWith(env, "docker", args...)
}

func (s *Store) restoreDatabaseState(j *restoreJournal) (*restoreDatabases, error) {
	args, env, err := s.restoreSQLCommand(j, "database-state", inspectRestoreDatabases)
	if err != nil {
		return nil, err
	}
	run := s.run
	if run.Env == nil {
		run.Env = os.Environ()
	}
	run.Env = append(append([]string(nil), run.Env...), env...)
	out, err := run.Capture("docker", args...)
	if err != nil {
		return nil, fmt.Errorf("cannot inspect restore database identities: %w", err)
	}
	var state restoreDatabases
	if err := json.Unmarshal([]byte(out), &state); err != nil {
		return nil, fmt.Errorf("invalid database identity response: %w", err)
	}
	for _, database := range []*restoreDatabase{state.Live, state.Staged, state.Old} {
		if database != nil && !restoreOIDPattern.MatchString(database.OID) {
			return nil, fmt.Errorf("invalid database identity response")
		}
	}
	return &state, nil
}

func (s *Store) swapRestoreDatabase(j *restoreJournal, rollback bool) error {
	state, err := s.restoreDatabaseState(j)
	if err != nil {
		return err
	}
	original := state.Live != nil && state.Live.OID == j.OriginalOID &&
		state.Staged != nil && state.Staged.OID == j.StagedOID && state.Staged.Tag == j.tag() && state.Old == nil
	swapped := state.Live != nil && state.Live.OID == j.StagedOID && state.Live.Tag == j.tag() &&
		state.Old != nil && state.Old.OID == j.OriginalOID && state.Staged == nil
	step, script := "swap-db", swapRestoreDatabases
	if rollback {
		step = "rollback-db"
		switch {
		case original:
			script = reconnectOriginalDatabase
		case swapped:
			script = rollbackRestoreDatabases
		default:
			return fmt.Errorf("database identities do not match either side of the interrupted cutover; no database was renamed")
		}
	} else if !original {
		return fmt.Errorf("database identities changed before cutover; no database was renamed")
	}
	if err := s.runRestoreSQL(j, step, script); err != nil {
		return fmt.Errorf("database cutover/recovery failed: %w", err)
	}
	return nil
}

func (s *Store) snapshotRestoreFiles(j *restoreJournal) error {
	if err := s.requireLiveFiles(true); err != nil {
		return err
	}
	if err := s.createRestoreVolume(j, j.oldVolume()); err != nil {
		return err
	}
	args := s.restoreHelperArgs(j, "snapshot-files", "none",
		"--mount", restoreMount("volume", s.Project+"_minio-data", "/live", true),
		"--mount", restoreMount("volume", j.oldVolume(), "/previous", false))
	args = append(args, s.Image, "sh", "-c", `set -eu
mkdir /previous/files
cp -a /live/. /previous/files/
sync -f /previous`)
	if err := s.run.Run("docker", args...); err != nil {
		return fmt.Errorf("cannot preserve the current uploaded files: %w", err)
	}
	return nil
}

func (s *Store) replaceRestoreFiles(j *restoreJournal, rollback bool) error {
	source, step := j.stageVolume(), "replace-files"
	if rollback {
		source, step = j.oldVolume(), "rollback-files"
	}
	if err := s.requireRestoreVolume(j, source); err != nil {
		return err
	}
	if err := s.requireLiveFiles(false); err != nil {
		return err
	}
	args := s.restoreHelperArgs(j, step, "none",
		"--mount", restoreMount("volume", source, "/source", true),
		"--mount", restoreMount("volume", s.Project+"_minio-data", "/live", false))
	args = append(args, s.Image, "sh", "-c", replaceRestoreFiles)
	if err := s.run.Run("docker", args...); err != nil {
		return fmt.Errorf("uploaded-files replacement/recovery failed: %w", err)
	}
	return nil
}

const validateRestoreFiles = `import gzip
import os
from pathlib import PurePosixPath
import tarfile

path = "/restore/files.tar.gz"
with gzip.open(path, "rb") as stream:
    while stream.read(1024 * 1024):
        pass
with tarfile.open(path, "r:gz") as archive:
    members = archive.getmembers()
    for member in members:
        name = PurePosixPath(member.name)
        if not member.name or name.is_absolute() or ".." in name.parts or not (member.isfile() or member.isdir()):
            raise ValueError("unsafe files archive entry: " + repr(member.name))
    os.mkdir("/restore/files", 0o700)
    archive.extractall("/restore/files", members=members, numeric_owner=True)
os.sync()
`

const replaceRestoreFiles = `set -eu
[ -d /source/files ]
for entry in /live/* /live/.[!.]* /live/..?*; do
  if [ -e "$entry" ] || [ -L "$entry" ]; then
    rm -rf -- "$entry"
  fi
done
cp -a /source/files/. /live/
sync -f /live`

const restorePSQL = `psql -X -q -d postgres -v ON_ERROR_STOP=1 \
  -v live="$RESTORE_DB" -v staged="$RESTORE_STAGE" -v old="$RESTORE_OLD" \
  -v tag="$RESTORE_TAG" -v original_oid="$RESTORE_ORIGINAL_OID" -v staged_oid="$RESTORE_STAGED_OID"`

const inspectRestoreDatabases = `set -eu
` + restorePSQL + ` -A -t <<'SQL'
SELECT json_build_object(
  'live', (SELECT json_build_object('oid', oid::text, 'tag', COALESCE(shobj_description(oid, 'pg_database'), '')) FROM pg_database WHERE datname = :'live'),
  'staged', (SELECT json_build_object('oid', oid::text, 'tag', COALESCE(shobj_description(oid, 'pg_database'), '')) FROM pg_database WHERE datname = :'staged'),
  'old', (SELECT json_build_object('oid', oid::text, 'tag', COALESCE(shobj_description(oid, 'pg_database'), '')) FROM pg_database WHERE datname = :'old'));
SQL`

const createRestoreDatabase = `set -eu
createdb --template=template0 -- "$RESTORE_STAGE" "$RESTORE_TAG"
pg_restore --exit-on-error --single-transaction --no-owner --no-privileges --dbname="$RESTORE_STAGE" /restore/database.dump`

const assertOriginalDatabase = `SELECT 1 / (
  (SELECT count(*) = 1 FROM pg_database WHERE datname = :'live' AND oid::text = :'original_oid') AND
  (SELECT count(*) = 1 FROM pg_database WHERE datname = :'staged' AND oid::text = :'staged_oid' AND shobj_description(oid, 'pg_database') = :'tag') AND
  NOT EXISTS (SELECT FROM pg_database WHERE datname = :'old'))::int;
`

const assertSwappedDatabase = `SELECT 1 / (
  (SELECT count(*) = 1 FROM pg_database WHERE datname = :'live' AND oid::text = :'staged_oid' AND shobj_description(oid, 'pg_database') = :'tag') AND
  (SELECT count(*) = 1 FROM pg_database WHERE datname = :'old' AND oid::text = :'original_oid') AND
  NOT EXISTS (SELECT FROM pg_database WHERE datname = :'staged'))::int;
`

const swapRestoreDatabases = `set -eu
` + restorePSQL + ` <<'SQL'
` + assertOriginalDatabase + `ALTER DATABASE :"live" ALLOW_CONNECTIONS false;
ALTER DATABASE :"staged" ALLOW_CONNECTIONS false;
SELECT pg_terminate_backend(pid, 5000) FROM pg_stat_activity WHERE datname IN (:'live', :'staged');
SELECT 1 / (count(*) = 0)::int FROM pg_stat_activity WHERE datname IN (:'live', :'staged');
BEGIN;
ALTER DATABASE :"live" RENAME TO :"old";
ALTER DATABASE :"staged" RENAME TO :"live";
COMMIT;
ALTER DATABASE :"live" ALLOW_CONNECTIONS true;
SQL`

const rollbackRestoreDatabases = `set -eu
` + restorePSQL + ` <<'SQL'
` + assertSwappedDatabase + `ALTER DATABASE :"live" ALLOW_CONNECTIONS false;
ALTER DATABASE :"old" ALLOW_CONNECTIONS false;
SELECT pg_terminate_backend(pid, 5000) FROM pg_stat_activity WHERE datname IN (:'live', :'old');
SELECT 1 / (count(*) = 0)::int FROM pg_stat_activity WHERE datname IN (:'live', :'old');
BEGIN;
ALTER DATABASE :"live" RENAME TO :"staged";
ALTER DATABASE :"old" RENAME TO :"live";
COMMIT;
ALTER DATABASE :"live" ALLOW_CONNECTIONS true;
SQL`

const reconnectOriginalDatabase = `set -eu
` + restorePSQL + ` <<'SQL'
` + assertOriginalDatabase + `ALTER DATABASE :"live" ALLOW_CONNECTIONS true;
SQL`

const stopRestoreDatabaseHelpers = `set -eu
` + restorePSQL + ` <<'SQL'
SELECT pg_terminate_backend(pid, 5000) FROM pg_stat_activity WHERE application_name = :'tag' AND pid <> pg_backend_pid();
SELECT 1 / (count(*) = 0)::int FROM pg_stat_activity WHERE application_name = :'tag' AND pid <> pg_backend_pid();
SQL`

const dropPreviousDatabase = `set -eu
` + restorePSQL + ` <<'SQL'
SELECT 1 / ((SELECT count(*) = 1 FROM pg_database WHERE datname = :'old' AND oid::text = :'original_oid'))::int;
DROP DATABASE :"old" WITH (FORCE);
SQL`

const dropStagedDatabase = `set -eu
` + restorePSQL + ` <<'SQL'
SELECT 1 / ((SELECT count(*) = 1 FROM pg_database WHERE datname = :'staged' AND shobj_description(oid, 'pg_database') = :'tag'
  AND (:'staged_oid' = '' OR oid::text = :'staged_oid')))::int;
DROP DATABASE :"staged" WITH (FORCE);
SQL`
