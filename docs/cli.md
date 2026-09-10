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
`care` acts on the **install dir** in the **current directory** (the folder with
`docker-compose.yml`). Override with `CARE_DESKTOP_DIR=/path/to/install-dir`.

```bash
cd care-desktop/deployments    # a valid install dir: it holds docker-compose.yml
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
| `care options` | List the staff roles and facility types this install accepts. Read-only. |
| `care restore <dump> [files.tar.gz]` | Restore a backup: drop + re-create the DB from `<dump>`, and (if a `files-*.tar.gz` is given, or auto-paired by timestamp) restore the uploaded files. **Replaces current data.** |
| `care uninstall [--images] [--backups] --yes` | Remove everything: containers, network, **all data volumes**, the installed files, the downloaded source, and the **trusted CA cert from this server's keychain**. `--images` also removes the Docker images; `--backups` also deletes the backup folder. Requires `--yes`. |

## Useful environment variables

| Variable | Example | Effect |
|---|---|---|
| `CARE_DESKTOP_DIR` | `/srv/care-desktop` | Use a folder other than the current dir. |
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

Example — check the role and facility type names the clinic details screen offers:
```bash
care options
```

## Notes
- Every command streams its progress to the terminal and exits non-zero on failure
  (so it's CI/script friendly).
- The clinic's facility and staff are entered once, on the second setup screen,
  and written by `scripts/clinic_seed.py` running inside the backend container
  through CARE's `load_fixtures` as the `admin` the installer created. It writes
  through CARE's own API, so every validation runs; it never touches the admin
  password, and it needs `DEBUG` to stay off. There is deliberately no way to
  re-run it afterwards — once CARE is up, facilities and staff are managed from
  inside CARE.
- The same step runs `sync_permissions_roles` and `sync_valueset` first (celery-beat
  runs them too, but only when it boots, which races a first install), then loads
  CARE's bundled questionnaires and report templates from `data/*.json`. Those load
  *after* the facility and staff commit, so a bad entry in a shipped file cannot
  roll back what the operator typed.
- That screen ships its own copy of the role and facility type lists, because it
  runs before there is a backend to ask. `care options` is how you check that copy
  still matches this install.
- `care restore` is destructive: it drops the current database before loading the
  dump. It stops the app services during the swap and restarts them after. Take a
  fresh `care backup-now` first if you're unsure.
- `care uninstall` deletes the data volumes, so it **won't run without `--yes`** —
  without it, it just prints what it would remove. Backups are kept unless you add
  `--backups`. Run against `deployments/`, it cleans Docker + the clones but leaves
  the source checkout itself in place. It does not rename the computer back. It **removes
  the clinic's CA cert(s) from *this* machine's trust store**: the root captured this
  run (by fingerprint) *and* any left by earlier installs, matched on the
  `CARE Desktop Local CA` common name our own Caddyfile sets, since setup mints a fresh
  root each time. Nothing else is touched, and if it can't remove them it says so
  rather than reporting a clean uninstall. Other devices keep their copy and must
  remove it manually.
- `care` never passes `-v` to `docker compose`, so **data volumes always survive**
  stop/start/rebuild. The only way to delete data is to remove the volumes yourself.
- The CLI and the desktop app are interchangeable — you can set up with one and
  manage with the other (they read the same install dir + config).
