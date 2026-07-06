# The `care` command-line tool

`care` is the terminal interface to the same engine the desktop app uses. It's for
developers and headless servers; non-technical staff use the app instead.

## Getting it
Build the binary once (needs Go — see [building.md](building.md)):
```bash
cd care-desktop/app
go build -o /usr/local/bin/care ./cmd/care    # or ~/.local/bin on Linux
```
Or run without building: `go run ./app/cmd/care <command>` from the repo root.

## Where it runs
`care` acts on the **kit** in the **current directory** (the folder with
`docker-compose.yml`). Override with `CARE_DESKTOP_DIR=/path/to/kit`.

```bash
cd care-desktop        # the repo root is a valid kit
care status
```

## Commands

| Command | What it does |
|---|---|
| `care setup` | One-time: generate a secret, clone + build the backend and frontend images, set up `care.local`. |
| `care start` | Ensure images exist → `docker compose up -d` → run migrations → create the default `admin`. |
| `care stop` | Stop the containers. **All data is kept.** |
| `care restart` | Restart the running containers. |
| `care rebuild-backend` | Rebuild the backend image from new code, recreate the Django/celery services, migrate. |
| `care rebuild-frontend` | Rebuild the frontend image (after editing `frontend.env`). |
| `care status` | Print each container's service + state. |
| `care backup-now` | Write an immediate database dump into the backup folder. |
| `care list-backups` | List the restorable points in the backup folder, newest first. |
| `care restore <dump> [files.tar.gz]` | Restore a backup: drop + re-create the DB from `<dump>`, and (if a `files-*.tar.gz` is given, or auto-paired by timestamp) restore the uploaded files. **Replaces current data.** |
| `care uninstall [--images] [--backups] --yes` | Remove everything: containers, network, **all data volumes**, the installed files, and the downloaded source. `--images` also removes the Docker images; `--backups` also deletes the backup folder. Requires `--yes`. |

## Useful environment variables

| Variable | Example | Effect |
|---|---|---|
| `CARE_DESKTOP_DIR` | `/srv/care-desktop` | Use a kit folder other than the current dir. |
| `BACKUP_DIR` | `/mnt/usb/care-backups` | Where backups go (default `~/Desktop/care-db-backups`). |
| `CARE_ADMIN_PASSWORD` | `s3cret` | Password for the first `admin` user (default `admin`). |
| `CARE_NO_MDNS` | `1` | Skip the hostname rename (use the server IP instead). |
| `CARE_BE_REF` / `CARE_FE_REF` | `v25.1.0` | Build a specific git ref instead of `develop`. |

Example — first run on a Linux server, backups to a USB drive:
```bash
cd care-desktop
BACKUP_DIR=/mnt/usb/care-backups CARE_ADMIN_PASSWORD=changeme care setup
BACKUP_DIR=/mnt/usb/care-backups care start
```

Example — restore the latest backup on this box:
```bash
cd care-desktop
care list-backups                      # copy the dump name you want
care restore care-20260701-020000.dump # DB + same-timestamp files, if present
```

## Notes
- Every command streams its progress to the terminal and exits non-zero on failure
  (so it's CI/script friendly).
- `care restore` is destructive: it drops the current database before loading the
  dump. It stops the app services during the swap and restarts them after. Take a
  fresh `care backup-now` first if you're unsure.
- `care uninstall` deletes the data volumes, so it **won't run without `--yes`** —
  without it, it just prints what it would remove. Backups are kept unless you add
  `--backups`. Run from the repo root, it cleans Docker + the clones but leaves the
  source checkout itself in place. It does not rename the computer back.
- `care` never passes `-v` to `docker compose`, so **data volumes always survive**
  stop/start/rebuild. The only way to delete data is to remove the volumes yourself.
- The CLI and the desktop app are interchangeable — you can set up with one and
  manage with the other (they read the same kit + config).
