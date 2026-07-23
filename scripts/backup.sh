#!/bin/sh
# Daily backup sidecar: dump the DB + tar uploaded files into /backups, prune old.
# If /keys/backup-cert.pem exists, dumps are streamed through `openssl cms` -> .enc
# (sidecar holds only the public cert). No cert = plaintext.
set -eu

BACKUP_DIR=/backups
MINIO_DIR=/minio-data
CERT=/keys/backup-cert.pem
RET="${DB_BACKUP_RETENTION_PERIOD:-14}"
DB_HOST="${POSTGRES_HOST:-db}"
DB_PORT="${POSTGRES_PORT:-5432}"
DB_USER="${POSTGRES_USER:-postgres}"
DB_NAME="${POSTGRES_DB:-care}"
export PGPASSWORD="${POSTGRES_PASSWORD:-postgres}"

# seal: stdin -> encrypted CMS blob at $1 (streamed, no full-file buffering).
seal() {
	openssl cms -encrypt -binary -aes-256-cbc -stream -outform DER -out "$1" "$CERT"
}

run_backup() {
	ts=$(date +%Y%m%d-%H%M%S)
	if [ -f "$CERT" ]; then enc=" (encrypted)"; ext=".enc"; else enc=""; ext=""; fi

	# DB (critical): write-then-rename so a crash leaves no truncated-but-valid dump.
	dump="$BACKUP_DIR/care-$ts.dump$ext"
	echo "[backup] $ts pg_dump $DB_NAME$enc"
	if [ -f "$CERT" ]; then
		( set -o pipefail 2>/dev/null || true
		  pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -Fc | seal "$dump.partial" )
	else
		pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -Fc -f "$dump.partial"
	fi
	mv "$dump.partial" "$dump"

	# uploaded files (best-effort: a benign tar failure just skips this cycle).
	if [ -d "$MINIO_DIR" ]; then
		arch="$BACKUP_DIR/files-$ts.tar.gz$ext"
		echo "[backup] $ts tar uploaded files$enc"
		if [ -f "$CERT" ]; then
			tar -czf - -C "$MINIO_DIR" . 2>/dev/null | seal "$arch" || echo "[backup] files archive skipped"
		else
			tar -czf "$arch" -C "$MINIO_DIR" . 2>/dev/null || echo "[backup] files archive skipped"
		fi
	fi

	# prune plaintext + .enc + stray .partial past retention
	find "$BACKUP_DIR" -name 'care-*.dump*' -mtime +"$RET" -delete 2>/dev/null || true
	find "$BACKUP_DIR" -name 'files-*.tar.gz*' -mtime +"$RET" -delete 2>/dev/null || true
	echo "[backup] done (kept ${RET} days)"
}

if [ -f "$CERT" ]; then
	echo "[backup] sidecar started; ENCRYPTED backups -> Desktop (retention ${RET}d)"
else
	echo "[backup] sidecar started; backups -> Desktop (retention ${RET}d)"
fi
while true; do
	run_backup || echo "[backup] FAILED - will retry next cycle"
	sleep 86400
done
