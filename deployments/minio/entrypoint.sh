#!/bin/sh
set -e

PATIENT="${FILE_UPLOAD_BUCKET:-patient-bucket}"
FACILITY="${FACILITY_S3_BUCKET:-facility-bucket}"

silo server /data --console-address ":9001" &
SILO_PID=$!

trap 'kill -TERM "$SILO_PID" 2>/dev/null' TERM INT

echo "[silo] waiting for server..."
until curl -sf http://localhost:9000/minio/health/ready >/dev/null 2>&1; do
	sleep 2
done

mc alias set local http://localhost:9000 "${MINIO_ROOT_USER:?MINIO_ROOT_USER not set}" "${MINIO_ROOT_PASSWORD:?MINIO_ROOT_PASSWORD not set}"

mc mb -p "local/$PATIENT"
mc mb -p "local/$FACILITY"

mc anonymous set download "local/$FACILITY"
echo "[silo] buckets ready: $PATIENT (private), $FACILITY (public read)"

wait "$SILO_PID"
