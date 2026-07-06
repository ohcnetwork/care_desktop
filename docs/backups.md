# Backups & restore

CARE Desktop backs up **automatically, every day**, with no setup. This page covers
where backups go, how to restore them, and how to keep them safe.

---

## What's backed up

A dedicated `backup` container runs continuously and, every 24 hours (and once at
startup), writes **two files** with a timestamp:

| File | Contents |
|---|---|
| `care-<timestamp>.dump` | the full PostgreSQL database (patient records, users, everything) — `pg_dump -Fc` |
| `files-<timestamp>.tar.gz` | the uploaded files from MinIO (X-rays, documents, logos) |

Both are needed for a full restore — the database alone won't bring back an X-ray.

> On-demand: click **Backup now** in the app, or run `care backup-now`. That writes
> an immediate `care-manual-<timestamp>.dump`.

---

## Where they go

- **Default:** `~/Desktop/care-db-backups/`
- **Chosen:** whatever folder you picked in the installer's "Backup location" step
  (or `BACKUP_DIR` for the CLI).

**Retention:** controlled by `DB_BACKUP_RETENTION_PERIOD` in `backend.env` (default
**14** days). Older `care-*.dump` and `files-*.tar.gz` are pruned automatically.

> ⚠️ **Put backups on a separate drive.** Point the backup folder at a **USB or
> external drive** (or copy it there regularly). If the server's disk dies, backups
> on that same disk die with it. The backup *and* the data being on one disk is not a
> backup — it's a single point of failure.

---

## Restore a backup

Restore is **built in** — you don't run the SQL by hand. It stops the app services,
drops + re-creates the database from the dump, restores the uploaded files (when the
backup has them), then brings CARE back up and migrates.

> ⚠️ **Restoring replaces the current data and can't be undone.** Take a fresh
> **Backup now** first if you're unsure.

### In the app

Open the control panel → **Restore from a backup**. Pick a point from the dropdown
(each is labelled with its date, whether it's a *daily* or *manual* backup, and
whether it includes files), click **Restore**, and confirm. Progress streams to the
log; the app restarts itself when it's done.

### From the CLI

```bash
care list-backups                      # newest first; copy the dump name
care restore care-YYYYMMDD-HHMMSS.dump  # DB + same-timestamp files, if present
```

- The matching `files-<timestamp>.tar.gz` is **paired automatically** by timestamp.
  To restore a different pair (or DB only), pass the files archive explicitly (or a
  dump that has none, like a manual `care-manual-*.dump`):
  ```bash
  care restore care-20260701-020000.dump files-20260701-020000.tar.gz
  ```
- Both the app and the CLI restore the same way — set up with one, restore with the other.

> Match the **database** dump and the **files** archive from the **same timestamp**
> for a consistent restore (the automatic pairing does this for you).

<details>
<summary>Manual restore (fallback, if you ever need it)</summary>

The built-in restore does exactly this. Stop the app first so nothing is writing:

```bash
care stop && docker compose -p care-desktop up -d db

docker compose -p care-desktop exec -T db psql -U postgres -c "DROP DATABASE IF EXISTS care;"
docker compose -p care-desktop exec -T db psql -U postgres -c "CREATE DATABASE care;"
cat ~/Desktop/care-db-backups/care-YYYYMMDD-HHMMSS.dump | \
  docker compose -p care-desktop exec -T db pg_restore -U postgres -d care

# files: extract the archive into the MinIO volume (care-desktop_minio-data)
docker run --rm \
  -v care-desktop_minio-data:/data \
  -v ~/Desktop/care-db-backups:/backup \
  alpine sh -c 'cd /data && tar -xzf /backup/files-YYYYMMDD-HHMMSS.tar.gz'

care start
```

</details>

---

## Moving the whole clinic to a new computer

1. On the old server: take a fresh backup (**Backup now**), copy the backup folder to a USB.
2. On the new server: install CARE Desktop (it builds a fresh, empty stack), and point its
   backup folder at the USB (installer step 5, or `BACKUP_DIR` for the CLI).
3. Restore that backup — **Restore from a backup** in the app, or `care restore <dump>`.
   (Restore stops and restarts the stack itself.)

Because patient data lives in the Docker **volumes** (captured by the backups), this
moves everything — records and files — to the new box.
