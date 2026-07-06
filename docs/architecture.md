# Architecture — how CARE Desktop works

This explains the moving parts: the app, the engine, the container stack, and how
a request flows. For *changing* settings see [configuration.md](configuration.md).

---

## Two layers

### 1. The control layer (one Go program)
A single Go codebase (`app/`) ships in two forms that share one **engine**:

- **Desktop app** (Wails) — a native window with an installer wizard and a control
  panel. For non-technical staff.
- **`care` CLI** — the same actions from a terminal. For developers/servers.

The **engine** (`app/internal/care/`) is plain Go that shells out to `docker` and
`git`. It has no GUI dependency, so the app and CLI can never drift. It replaced
the old `care.sh` bash script entirely — there is **no shell dependency** on any OS.

What the engine does:

| Action | What happens |
|---|---|
| `setup` | generate a Django secret → clone + build the backend and frontend images → set up `care.local` |
| `start` | ensure images exist → `docker compose up -d` → run DB migrations → create the default admin |
| `stop` / `restart` | stop / restart containers (data always preserved) |
| `rebuild-backend` | rebuild the backend image from new code, recreate + migrate |
| `rebuild-frontend` | rebuild the frontend image (Vite bakes settings at build time) |
| `status` | report each container's state |
| `backup-now` | write an immediate database dump |
| `list-backups` | list the restorable points in the backup folder |
| `restore` | stop app services → drop + re-create the DB from a chosen dump → (optionally) overwrite the MinIO volume from the matching files archive → bring the stack back up + migrate |
| `uninstall` | `compose down -v` (containers + network + **data volumes**) → optionally remove images → delete the clones + kit dir → optionally delete backups. The app also clears its config + login-item so the next launch is a fresh first-run |

### 2. The runtime layer (Docker containers)
The actual CARE stack, defined in `docker-compose.yml`. Project name is
**`care-desktop`** so it never collides with another CARE stack on the same machine.

| Service | Image | Role |
|---|---|---|
| `db` | `postgres:17-alpine` | the database (patient records, etc.) |
| `redis` | `redis:8-alpine` | cache + Celery task queue |
| `minio` | `minio/minio` | S3-compatible file storage (uploads, X-rays, logos) |
| `backend` | `care:clinic` (built) | the Django API + `/admin` |
| `celery-worker` | `care:clinic` | background jobs |
| `celery-beat` | `care:clinic` | scheduled jobs |
| `frontend` | `care_fe:clinic` (built) | the React app (served by nginx) |
| `caddy` | `caddy:2` | reverse proxy — the single front door on port 80 |
| `backup` | `postgres:17-alpine` | daily DB dump + file archive |

Only **one port** is exposed to the WiFi:
- **80** — everything, via Caddy: the app, the API, and file upload/download.

Everything else (postgres 5432, redis 6379, the backend's 9000, MinIO's 9000) is
private inside the Docker network. Files reach the browser through Caddy on port 80
too (see below), so there's a single port to bind — no conflicts with other
services, and nothing extra to open on the firewall.

---

## How a request flows

A nurse opens `http://care.local` on her phone:

1. **Name → address.** The phone asks the LAN "who is `care.local`?" — **mDNS**
   (Bonjour/Avahi) answers with the server's IP. No internet, no router config.
2. **Phone → Caddy.** It connects to the server on **port 80**, hitting Caddy (the
   reverse proxy / single front door).
3. **Caddy routes by path** (`Caddyfile`):
   - `/api/*`, `/admin/*`, `/static/*`, `/ping/*`, `/health/*` → **backend:9000**
   - `/patient-bucket/*`, `/facility-bucket/*` → **minio:9000** (files)
   - everything else → **frontend:80** (the React app)
4. **The React app runs in the phone** and calls `/api/...` at the **same address**
   (`http://care.local`) — so it's *same-origin*, and there's **no CORS** to deal with.
5. **Backend talks to** postgres (data) + redis (cache/jobs) and returns JSON.
6. **Files** (X-rays, documents) use **presigned URLs**: the browser uploads/
   downloads at `http://care.local/patient-bucket/...`, which Caddy routes to MinIO.
   Same origin as the app, on the same port 80 — no CORS, no extra port. (This is why
   `BUCKET_EXTERNAL_ENDPOINT` must be a name every device can reach — never
   `localhost`.) The bucket name stays in the path and isn't rewritten, so MinIO's
   SigV4 signature check on the presigned URL still passes.

---

## Plain HTTP on the LAN (`clinic_settings.py`)

CARE's production settings assume HTTPS. On a trusted offline LAN we use plain
`http://`, so `clinic_settings.py` imports the production settings and relaxes only
the HTTPS-only guards:

```python
from config.settings.deployment import *
DEBUG = False                   # never debug on a clinic box
SECURE_SSL_REDIRECT = False     # don't bounce LAN http → https
SESSION_COOKIE_SECURE = False   # let /admin + CSRF cookies work over http
CSRF_COOKIE_SECURE = False
SECURE_HSTS_SECONDS = 0
CORS_ALLOW_ALL_ORIGINS = True   # the proxy is same-origin anyway
```

It's mounted into the backend container at `/settings/` and selected with
`DJANGO_SETTINGS_MODULE=clinic_settings`. **The core CARE app is never modified.**

---

## Why the frontend is built locally

The frontend (`care_fe`) is a **Vite** app: it **bakes** `REACT_CARE_API_URL` into
the static files at *build* time, and its build validator rejects an empty value.
The official prebuilt image points at CARE's cloud API — useless for an offline
clinic. So setup **builds the frontend once**, pinned to `http://care.local`. The
backend, by contrast, reads its settings at runtime, so it could use any image — but
for now both are built from source (branch `develop`).

See [configuration.md](configuration.md#versionsenv) for pinning versions.

---

## Where things live on the server

| Path (macOS shown) | What |
|---|---|
| `~/Library/Application Support/care-desktop/config.json` | the app's saved choices (setup done, install/backup folders) |
| `~/Library/Application Support/care-desktop/kit/` | the unpacked deployment kit + the `care`/`care_fe` clones |
| `~/Desktop/care-db-backups/` (default) | daily backups (override in the installer) |
| Docker named volumes | `postgres-data`, `redis-data`, `minio-data`, `caddy-*` — the actual data |

On Linux the config dir is `~/.config/care-desktop/`; on Windows it's
`%AppData%\care-desktop\`.

> **Patient data lives in the Docker volumes**, not in the install folder. The
> install folder only holds the kit + source clones. This is why moving/clearing the
> install folder never loses data — but you must still back up the volumes (the
> `backup` container does this automatically).
