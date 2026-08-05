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
| `setup` | validate the clinic's web address → generate a Django secret → clone + build the backend and frontend images |
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
| `caddy` | `caddy:2` + Coraza WAF + Cloudflare DNS (built) | reverse proxy — the single HTTPS front door on port 443 |
| `backup` | `postgres:17-alpine` | daily DB dump + file archive |

Only **one port** is exposed to the WiFi:
- **443** — everything, via Caddy: the app, the API, and file upload/download.

There is no port 80 at all. A plaintext door can be intercepted and stripped by anyone
on the same network, which would undo the encryption it hands off to — so it isn't
served or published. See [tls.md](tls.md).

Everything else (postgres 5432, redis 6379, the backend's 9000, MinIO's 9000) is
private inside the Docker network. Files reach the browser through Caddy on port 443
too (see below), so there's a single port to bind — no conflicts with other
services, and nothing extra to open on the firewall.

---

## How a request flows

A nurse opens `https://clinic.yourdomain.com` on her phone:

1. **Name → address.** The phone looks the name up in public DNS and gets the server's
   **LAN IP** (e.g. `192.168.1.50`) — an address that only means anything inside the
   building. The name is public; the destination is not.
2. **Phone → Caddy.** It connects to the server on **port 443** over TLS, hitting Caddy
   (the reverse proxy / single front door). Caddy presents a Let's Encrypt certificate
   it obtained and renews itself, so the padlock works with nothing installed.
3. **Caddy routes by path** (`Caddyfile`):
   - `/api/*`, `/static/*`, `/ping/*`, `/health/*` → **backend:9000**
   - `/patient-bucket/*`, `/facility-bucket/*` → **minio:9000** (files)
   - everything else → **frontend:80** (the React app)
4. **The React app runs in the phone** and calls `/api/...` at the **same address**
   (`https://clinic.yourdomain.com`) — so it's *same-origin*, and there's **no CORS**.
5. **Backend talks to** postgres (data) + redis (cache/jobs) and returns JSON.
6. **Files** (X-rays, documents) use **presigned URLs**: the browser uploads/
   downloads at `https://clinic.yourdomain.com/patient-bucket/...`, which Caddy routes
   to MinIO. Same origin, same port — no CORS, no extra port. (This is why
   `BUCKET_EXTERNAL_ENDPOINT` must be a name every device can reach — never
   `localhost`.) The bucket name stays in the path and isn't rewritten, so MinIO's
   SigV4 signature check on the presigned URL still passes.

---

## HTTPS everywhere, on the clinic's own domain

CARE is served only over HTTPS, at a domain the clinic owns, with a certificate Caddy
obtains and renews by itself. Clinic devices install nothing — the certificate chains
to a CA every browser already ships.

The name is public, but it resolves to the server's **LAN IP**, so traffic still never
leaves the building; only the certificate paperwork touches the internet. Because the
server sits behind the router's NAT with no port forwarding, the usual "let the CA
connect in and check" verification can't work — instead Caddy proves domain ownership
by writing a DNS record through the Cloudflare API (**DNS-01**), which needs only
outbound internet. Renewal starts 30 days before expiry, so the connection would have
to be down for a month before anything broke.

There is deliberately no plain-HTTP fallback. See [tls.md](tls.md).

## Django behind the proxy (`clinic_settings.py`)

CARE's production settings assume it is the thing terminating TLS. Here Caddy does,
and proxies plain HTTP inside the Docker network — so `clinic_settings.py` imports the
production settings and adjusts only what that changes:

```python
from config.settings.deployment import *
DEBUG = False                   # never debug on a clinic box
SECURE_SSL_REDIRECT = False     # Caddy owns the redirect; doing it here too loops
SECURE_PROXY_SSL_HEADER = ("HTTP_X_FORWARDED_PROTO", "https")
CORS_ALLOW_ALL_ORIGINS = True   # the proxy is same-origin anyway
```

`SECURE_PROXY_SSL_HEADER` is the load-bearing one: without it Django treats every
request as insecure and builds `http://` absolute URLs, which breaks presigned file
links and `/admin` redirects in ways that don't look like a TLS problem. It's safe to
trust because Caddy sets that header itself on every proxied request, and the backend
port is never published outside the compose network.

Session and CSRF cookies are marked `Secure`, so the browser will not send them over
plain http at all; CORS is shut (everything is one origin); and Caddy adds HSTS and
`nosniff` at the edge so they cover the React app and the file buckets too, not only
Django's own responses. The reasoning for each is in
[tls.md](tls.md#whats-hardened-and-why).

It's mounted into the backend container at `/settings/` and selected with
`DJANGO_SETTINGS_MODULE=clinic_settings`. **The core CARE app is never modified.**

---

## Why the frontend is built locally

The frontend (`care_fe`) is a **Vite** app: it **bakes** `REACT_CARE_API_URL` into
the static files at *build* time, and its build validator rejects an empty value.
The official prebuilt image points at CARE's cloud API — useless for an offline
clinic. So setup **builds the frontend once**, pinned to the clinic's own address —
which is why changing that address triggers a rebuild rather than a restart. The
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
