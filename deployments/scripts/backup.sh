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

# Temp files live in BACKUP_DIR, not /tmp: the final `mv` has to be
# same-filesystem to be atomic, so a crash can never leave a half-written file
# under a name the app would list as restorable. The leading dot keeps them out
# of the prune globs and out of the app's backup listing.
db_backup() {
	ts=$1
	plain="$BACKUP_DIR/.care-$ts.dump.tmp"
	sealed="$plain.enc"
	final="$BACKUP_DIR/care-$ts.dump.enc"

	echo "[backup] database: dumping $DB_NAME"
	if ! pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -Fc -f "$plain"; then
		echo "[backup] ERROR: pg_dump failed"
		rm -f "$plain"
		return 1
	fi

	if ! pg_restore --list "$plain" >/dev/null 2>&1; then
		echo "[backup] ERROR: dump failed verification (pg_restore --list)"
		rm -f "$plain"
		return 1
	fi
	echo "[backup] database: $(size_of "$plain") verified"

	if ! seal "$plain" "$sealed"; then
		echo "[backup] ERROR: encrypting the dump failed"
		rm -f "$plain" "$sealed"
		return 1
	fi
	rm -f "$plain"

	mv "$sealed" "$final"
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

	echo "[backup] files: archiving uploads"

	tar -czf "$plain" -C "$MINIO_DIR" . 2>/dev/null || true
	if [ ! -s "$plain" ] || ! tar -tzf "$plain" >/dev/null 2>&1; then
		echo "[backup] ERROR: files archive is missing or unreadable"
		rm -f "$plain"
		return 1
	fi
	echo "[backup] files: $(size_of "$plain") verified"

	if ! seal "$plain" "$sealed"; then
		echo "[backup] ERROR: encrypting the files archive failed"
		rm -f "$plain" "$sealed"
		return 1
	fi
	rm -f "$plain"

	mv "$sealed" "$final"
	echo "[backup] files: wrote $(basename "$final")"
}


prune() {
	find "$BACKUP_DIR" -name '.care-*.tmp*' -delete 2>/dev/null || true
	find "$BACKUP_DIR" -name '.files-*.tmp*' -delete 2>/dev/null || true

	if [ "$RET" -eq 0 ]; then
		echo "[backup] $RET_DESC"
		return 0
	fi
	echo "[backup] retention: deleting sets older than ${RET} days"
	find "$BACKUP_DIR" -name 'care-*.dump*' -mtime +"$RET" -delete 2>/dev/null || true
	find "$BACKUP_DIR" -name 'files-*.tar.gz*' -mtime +"$RET" -delete 2>/dev/null || true
}

run_backup() {
	ts=$(date +%Y%m%d-%H%M%S)
	echo "[backup] ===== backup set $ts (encrypted) ====="

	if ! db_backup "$ts"; then
		echo "[backup] backup set $ts: FAILED at the database step"
		return 1
	fi

	if ! files_backup "$ts"; then
		echo "[backup] backup set $ts: FAILED at the files step"
		return 1
	fi

	echo "[backup] backup set $ts: SUCCESS"
	prune
	echo "[backup] done"
}

echo "[backup] sidecar started; encrypted backups -> $BACKUP_DIR ($RET_DESC)"

while true; do
	if ! run_backup; then
		echo "[backup] retention skipped - every existing backup retained"
	fi
	sleep 86400
done
