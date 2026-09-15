#!/bin/sh
set -eu

BACKUP_DIR=/backups
MINIO_DIR=/minio-data
CERT=/keys/backup-cert.pem

RET="${DB_BACKUP_RETENTION_PERIOD:-0}"

if [ "$RET" -eq 0 ]; then
	RET_DESC="retention 0 - backups are kept forever"
else
	RET_DESC="retention ${RET}d"
fi

if [ ! -f "$CERT" ]; then
	echo "[backup] ERROR: $CERT not found - all backups must be encrypted, refusing to run"
	exit 1
fi

if [ -z "${POSTGRES_PASSWORD:-}" ]; then
	echo "[backup] ERROR: POSTGRES_PASSWORD is not set - refusing to run"
	exit 1
fi
export PGPASSWORD="$POSTGRES_PASSWORD"

DB_HOST="${POSTGRES_HOST:-db}"
DB_PORT="${POSTGRES_PORT:-5432}"
DB_USER="${POSTGRES_USER:-postgres}"
DB_NAME="${POSTGRES_DB:-care}"

# seal: plaintext file $1 -> encrypted CMS blob at $2.
seal() {
	openssl cms -encrypt -binary -aes-256-cbc -stream -outform DER -in "$1" -out "$2" "$CERT"
}

size_of() { du -h "$1" 2>/dev/null | cut -f1; }

remove_temp() {
	if ! rm -f "$@"; then
		echo "[backup] ERROR: removing temporary backup files failed: $*"
		return 1
	fi
}

require_new_paths() {
	for backup_path in "$@"; do
		if [ -e "$backup_path" ] || [ -L "$backup_path" ]; then
			echo "[backup] ERROR: refusing to overwrite existing backup: $backup_path"
			return 1
		fi
	done
}


db_backup() {
	ts=$1
	plain="$BACKUP_DIR/.care-$ts.dump.tmp"
	sealed="$plain.enc"
	final="$BACKUP_DIR/care-$ts.dump.enc"
	require_new_paths "$final" || return 1

	echo "[backup] database: dumping $DB_NAME"
	if ! pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -Fc -f "$plain"; then
		echo "[backup] ERROR: pg_dump failed"
		remove_temp "$plain"
		return 1
	fi

	if ! pg_restore --list "$plain" >/dev/null 2>&1; then
		echo "[backup] ERROR: dump failed verification (pg_restore --list)"
		remove_temp "$plain"
		return 1
	fi
	echo "[backup] database: $(size_of "$plain") verified"

	if ! seal "$plain" "$sealed"; then
		echo "[backup] ERROR: encrypting the dump failed"
		remove_temp "$plain" "$sealed"
		return 1
	fi
	if ! remove_temp "$plain"; then
		remove_temp "$sealed"
		return 1
	fi

	if ! mv "$sealed" "$final"; then
		echo "[backup] ERROR: publishing the dump failed"
		remove_temp "$sealed"
		return 1
	fi
	echo "[backup] database: wrote $(basename "$final")"
}

files_backup() {
	ts=$1
	if [ ! -d "$MINIO_DIR" ]; then
		echo "[backup] ERROR: $MINIO_DIR is not mounted - refusing to call this a complete backup"
		return 1
	fi
	plain="$BACKUP_DIR/.files-$ts.tar.gz.tmp"
	sealed="$plain.enc"
	final="$BACKUP_DIR/files-$ts.tar.gz.enc"
	require_new_paths "$final" || return 1

	echo "[backup] files: archiving uploads"

	if ! tar -czf "$plain" -C "$MINIO_DIR" . 2>/dev/null; then
		echo "[backup] ERROR: archiving the files failed"
		remove_temp "$plain"
		return 1
	fi
	if [ ! -s "$plain" ] || ! tar -tzf "$plain" >/dev/null 2>&1; then
		echo "[backup] ERROR: files archive is missing or unreadable"
		remove_temp "$plain"
		return 1
	fi
	echo "[backup] files: $(size_of "$plain") verified"

	if ! seal "$plain" "$sealed"; then
		echo "[backup] ERROR: encrypting the files archive failed"
		remove_temp "$plain" "$sealed"
		return 1
	fi
	if ! remove_temp "$plain"; then
		remove_temp "$sealed"
		return 1
	fi

	if ! mv "$sealed" "$final"; then
		echo "[backup] ERROR: publishing the files archive failed"
		remove_temp "$sealed"
		return 1
	fi
	echo "[backup] files: wrote $(basename "$final")"
}


prune() {
	if ! find "$BACKUP_DIR" -maxdepth 1 -type f \( -name '.care-*.tmp*' -o -name '.files-*.tmp*' \) -delete; then
		echo "[backup] ERROR: removing abandoned temporary backup files failed"
		return 1
	fi

	if [ "$RET" -eq 0 ]; then
		echo "[backup] $RET_DESC"
		return 0
	fi
	echo "[backup] retention: deleting sets older than ${RET} days"
	if ! find "$BACKUP_DIR" -maxdepth 1 -type f \( -name 'care-*.dump*' -o -name 'files-*.tar.gz*' \) -mtime +"$RET" -delete; then
		echo "[backup] ERROR: pruning old backups failed"
		return 1
	fi
}

run_backup() {
	ts=$(date +%Y%m%d-%H%M%S)
	echo "[backup] ===== backup set $ts (encrypted) ====="

	if ! require_new_paths "$BACKUP_DIR/care-$ts.dump.enc" "$BACKUP_DIR/files-$ts.tar.gz.enc"; then
		echo "[backup] backup set $ts: FAILED before writing; retention skipped"
		return 1
	fi

	if ! db_backup "$ts"; then
		echo "[backup] backup set $ts: FAILED at the database step; retention skipped"
		return 1
	fi

	if ! files_backup "$ts"; then
		echo "[backup] backup set $ts: FAILED at the files step; retention skipped"
		return 1
	fi

	if ! prune; then
		echo "[backup] backup set $ts: published, but cleanup FAILED"
		return 1
	fi
	echo "[backup] backup set $ts: SUCCESS"
	echo "[backup] done"
}

with_backup_lock() (
	if ! flock -x 9; then
		echo "[backup] ERROR: acquiring the backup lock failed"
		return 1
	fi
	"$@"
) 9>"$BACKUP_DIR/.backup.lock"

# One-shot mode, used by the app's "Backup now": database only, under the name
# the caller picked, then exit. Same dump/verify/seal/rename as the daily run.
if [ "${1:-}" = "once" ]; then
	with_backup_lock db_backup "${2:?no backup name given}"
	exit
fi

echo "[backup] sidecar started; encrypted backups -> $BACKUP_DIR ($RET_DESC)"

while true; do
	if ! with_backup_lock run_backup; then
		echo "[backup] backup cycle failed - see errors above"
	fi
	sleep 86400
done
