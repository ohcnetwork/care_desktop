#!/bin/sh
# Start MinIO, wait until ready, create the buckets CARE needs, then keep running.
set -e

minio server /data --console-address ":9001" &
MINIO_PID=$!

echo "[minio] waiting for server..."
until curl -sf http://localhost:9000/minio/health/ready >/dev/null 2>&1; do
	sleep 2
done

mc alias set local http://localhost:9000 "${MINIO_ACCESS_KEY:-minioadmin}" "${MINIO_SECRET_KEY:-minioadmin}"
mc mb -p local/patient-bucket || true
mc mb -p local/facility-bucket || true
# facility-bucket holds non-sensitive public assets (logos, etc.)
mc anonymous set public local/facility-bucket || true
echo "[minio] buckets ready"

wait "$MINIO_PID"
