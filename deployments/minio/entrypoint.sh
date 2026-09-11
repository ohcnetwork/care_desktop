#!/bin/sh
# Start MinIO, wait until ready, create the buckets CARE needs, then keep running.
set -e

PATIENT="${FILE_UPLOAD_BUCKET:-patient-bucket}"
FACILITY="${FACILITY_S3_BUCKET:-facility-bucket}"

minio server /data --console-address ":9001" &
MINIO_PID=$!

trap 'kill -TERM "$MINIO_PID" 2>/dev/null' TERM INT

echo "[minio] waiting for server..."
until curl -sf http://localhost:9000/minio/health/ready >/dev/null 2>&1; do
	sleep 2
done

mc alias set local http://localhost:9000 "${MINIO_ACCESS_KEY:-minioadmin}" "${MINIO_SECRET_KEY:-minioadmin}"

mc mb -p "local/$PATIENT"
mc mb -p "local/$FACILITY"

mc anonymous set download "local/$FACILITY"
echo "[minio] buckets ready: $PATIENT (private), $FACILITY (public read)"

wait "$MINIO_PID"
