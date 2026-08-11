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
| `uninstall` | `compose down -v` (containers + network + **data volumes**) → optionally remove images → delete the clones + kit dir → **remove the trusted CA cert from the server's keychain** → optionally delete backups. It also drops the `care.local` hosts line on every OS, and on Windows removes the `CARE Desktop *` firewall rules and reverts the network profile to Public. The app also clears its config + login-item so the next launch is a fresh first-run |

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
| `caddy` | `care-caddy:clinic` (Caddy + Coraza WAF, built) | reverse proxy + TLS terminator — the single front door, and the app-firewall on `/api` |
| `backup` | `postgres:17-alpine` | daily DB dump + file archive |

Two ports are exposed to the WiFi:
- **443** — the app, the API, and file upload/download, all over **HTTPS** via Caddy.
- **80** — only redirects to `https://` **and** serves the cert-trust bootstrap
  (`/setup` + `/root.crt`) so a device that doesn't trust us yet can still fetch the
  cert. Everything else on `:80` is a permanent redirect to `:443`.

Everything else (postgres 5432, redis 6379, the backend's 9000, MinIO's 9000) is
private inside the Docker network. Files reach the browser through Caddy too (same
origin, same port), so there's a single origin to bind — no conflicts with other
services, and only 80/443 to open on the firewall.

---

## How a request flows

A nurse opens `https://care.local` on her phone:

1. **Name → address.** The phone asks the LAN "who is `care.local`?" — **mDNS**
   (Bonjour/Avahi) answers with the server's IP. No internet, no router config. The
   app runs the advertiser in-process and **self-heals** it (see below).
2. **Phone → Caddy (TLS).** It connects to the server on **port 443** and completes a
   TLS handshake; Caddy presents the self-signed cert, which the device trusts because
   it installed the local CA once via `/setup`. (A stray `http://` hit on **:80** is
   redirected to `https://` — except `/setup` and `/root.crt`, which :80 serves so an
   untrusting device can still fetch the cert.)
3. **Caddy routes by path** (`Caddyfile`):
   - `/api/*` → **backend:9000**, but first through the **Coraza WAF** (OWASP CRS)
   - `/static/*`, `/ping/*`, `/health/*` → **backend:9000**
   - `/setup*`, `/root.crt` → the cert-trust bootstrap (static page + the CA file)
   - `/patient-bucket/*`, `/facility-bucket/*` → **minio:9000** (files)
   - everything else → **frontend:80** (the React app)
4. **The React app runs in the phone** and calls `/api/...` at the **same address**
   (`https://care.local`) — so it's *same-origin*, and there's **no CORS** to deal with.
5. **Backend talks to** postgres (data) + redis (cache/jobs) and returns JSON. Caddy
   terminates TLS and forwards to the backend over plain http inside the Docker
   network, adding `X-Forwarded-Proto: https` so Django still sees a secure request
   (secure cookies + CSRF work — see `clinic_settings.py` below).
6. **Files** (X-rays, documents) use **presigned URLs**: the browser uploads/
   downloads at `https://care.local/patient-bucket/...`, which Caddy routes to MinIO.
   Same origin as the app, on the same port — no CORS, no extra port. (This is why
   `BUCKET_EXTERNAL_ENDPOINT` must be a name every device can reach — never
   `localhost`.) The bucket name stays in the path and isn't rewritten, so MinIO's
   SigV4 signature check on the presigned URL still passes.

---

## HTTPS on the LAN (`clinic_settings.py`)

The clinic runs over `https://` even though it's an offline LAN, because modern
browsers gate key features behind a **secure context**: `navigator.mediaDevices`
(the camera used for document/QR scanning) and secure cookies only work on `https://`
or `localhost`. Plain http would break those on every phone. No public CA can sign a
`.local` name, so Caddy issues a **self-signed cert from a built-in local CA**
(`tls internal`) and devices trust that CA once (see the next section).

CARE's production settings assume HTTPS *and* a public cert, so `clinic_settings.py`
imports the production settings and relaxes only what a **self-signed** LAN cert needs:

```python
from config.settings.deployment import *
DEBUG = False                   # never debug on a clinic box
SECURE_SSL_REDIRECT = False     # Caddy already redirects http -> https; don't double it
SESSION_COOKIE_SECURE = True    # https-only origin
CSRF_COOKIE_SECURE = True
SECURE_HSTS_SECONDS = 0         # self-signed cert - don't HSTS-pin clients to https
# Caddy terminates TLS and forwards over http with this header, so Django still sees
# the original https request (request.is_secure(), secure cookies, CSRF):
SECURE_PROXY_SSL_HEADER = ("HTTP_X_FORWARDED_PROTO", "https")
CORS_ALLOW_ALL_ORIGINS = True   # the proxy is same-origin anyway
```

It's mounted into the backend container at `/settings/` and selected with
`DJANGO_SETTINGS_MODULE=clinic_settings`. **The core CARE app is never modified.**

---

## Certificate trust — the `/setup` bootstrap

A self-signed cert means every device shows a security warning until it trusts the
clinic's **root CA** once. Caddy serves a small bootstrap for exactly this, on **both**
http and https (an untrusting device can't open https yet, and iOS Safari
auto-upgrades http→https — so the bootstrap must answer on both):

- **`/setup`** — a static install page (`setup/index.html`) with a device picker
  (Windows / macOS / Linux / Android / iOS) that shows per-OS steps and a download
  button for the CA.
- **`/root.crt`** — the CA certificate itself, served with MIME
  `application/x-x509-ca-cert`. It's **gated**: a direct hit without the setup page's
  `?ok=1` marker (or a same-site `Referer`) is redirected back to `/setup`, so people
  go through the instructions instead of grabbing a raw file they don't know how to use.
- **`/setup/install-cert.sh`** and **`/setup/install-cert.ps1`**: one-step installers
  for laptops and desktops (see below). Served with `Content-Disposition: attachment`
  so a browser downloads them instead of rendering the source.

### One-step installers

Trusting a CA by hand is the worst part of onboarding a device: on Windows it's a
six-screen wizard, on macOS a keychain dialog plus a separate *Always Trust* toggle,
on Linux a `cp` into a distro-specific directory. So for **Windows, macOS, and Linux**
the setup page offers a script that does it in one run. Phones can't execute one, so
Android and iOS keep the manual flow only.

The engine renders both scripts on every `start`
(`app/internal/care/certscript.go`) into the `setup/` directory Caddy serves. Two
properties matter:

- **The certificate is embedded in the script, not fetched.** A device that doesn't
  trust the clinic yet cannot fetch over https, and an installer that disables TLS
  verification to bootstrap itself would be a bad habit to ship. The script carries the
  PEM inline, so it needs no network at all.
- **They're regenerated each start**, because deleting the `caddy-data` volume re-mints
  the root, and a stale script would install a CA the server no longer uses.

Each script prints the certificate's **SHA-256 fingerprint**, which the setup page also
shows, so the two can be compared before running it. The Unix script re-executes itself
under `sudo`, detects macOS vs Linux, handles both the Debian and RHEL anchor layouts,
and additionally loads the CA into the per-user **NSS** store when `certutil` is present
(Firefox and Chrome on Linux keep their own, separate from the system bundle). The
PowerShell script self-elevates through UAC and imports into `LocalMachine\Root`.

The CA is **renamed** in the Caddyfile's `pki` block so the trust prompt (e.g. the iOS
profile screen) reads **"CARE Desktop Local CA"** instead of Caddy's default
"Caddy Local Authority". This only affects a freshly-minted root — deleting the
`caddy-data` volume re-mints it.

**The server machine trusts its own cert automatically.** At the end of `start`, the
engine (`app/internal/care/trustca.go`) pulls the root out of the `caddy-data` volume
and installs it into the OS trust store — unprivileged first (login keychain / user
store), which on macOS usually succeeds outright. It's idempotent: if the cert is
already trusted it does nothing.

### One install, one approval

Two things must happen for the clinic to open in the **server's own** browser, and
both need administrator rights: the hosts entry above, and trusting the CA. Neither
can run before the stack is up (the CA doesn't exist until Caddy mints it into its
volume), so both land at the end of `start`. Running them separately would mean
two dialogs and two password prompts to finish one install.

So `ensureLocalAccess` (`app/internal/care/localaccess.go`) collects whatever is
still outstanding and runs it in a **single elevated call behind a single
confirmation**. Each step is tried unprivileged first and drops out if it succeeds
(or was already done), so the prompt lists only what genuinely needs admin, and a
repeat `start` asks for nothing at all. The approval is a **native dialog** in the
app and a `y/N` prompt in the CLI; elevation is `osascript` (macOS), `RunAs`/UAC
(Windows), or `pkexec` (Linux). It's best-effort throughout and never fails `start`:
by that point the clinic is already serving every other device.

**Uninstall removes it again.** The root lives in the `caddy-data` volume that
`compose down -v` destroys, so `uninstall` captures it **before** teardown, via
`exec cat`, falling back to `compose cp` if the container won't take an exec, because
there's only one attempt before the volume is gone.

It then removes **every** CARE root, not just that one: by **SHA-1 fingerprint** (the
id macOS `security` and Windows `certutil` both use) for the cert captured this run,
*and* by the `CARE Desktop Local CA` common name that our own Caddyfile's `pki` block
sets. The name sweep matters because setup mints a **fresh root every time**: matching
only the current fingerprint left every earlier install's root trusted forever. That CN
is ours by construction, so nothing else is ever touched. Each store is read first
(free, no admin) so an uninstall with nothing to remove raises no prompt, and a failure
is reported rather than swallowed. Only the server machine is cleaned up. Other devices
keep their copy and must remove it manually.

---

## mDNS advertising and self-heal

The `care.local` name is advertised by an **in-process** mDNS responder in the app (Go,
`app/internal/care/advertise.go`) — not a system daemon — so it works the same on every
OS the app runs on. Sleep/wake, WiFi flaps, or an IP change can silently stop it
answering, which shows up as `DNS_PROBE_FINISHED_NXDOMAIN` on client devices.

A watchdog in the app (`watchAdvertise`) re-advertises when it detects either the
server's **IP changed** or that `care.local` **stops resolving** (a debounced liveness
probe via the system resolver — two consecutive misses, ~60s, before it acts, to avoid
flapping). This keeps the name reachable across network hiccups without a manual
restart.

**The server's own browser is a separate problem on every OS.** A host's resolver
won't hand back a name that a *second* responder on the same machine advertises:
Windows has no mDNS resolver at all without Bonjour; macOS's `mDNSResponder` owns
`.local` and ignores multicast answers originating from its own addresses; Linux
only resolves `.local` when `nss-mdns` is installed and wired into `nsswitch.conf`.
The responder is reachable from *other* devices the whole time, just not from the
box running it.

That matters because the frontend has `https://care.local` baked in as its API base,
so an unresolvable name means every call fails with `ERR_NAME_NOT_RESOLVED` even if
you reach the app by IP, and since CARE gates its first paint on an API call the
page sits on its loading logo. So CARE writes a single hosts-file line,
`127.0.0.1 care.local` (`app/internal/care/hosts.go`, tagged `# care-desktop`,
removed on uninstall), on **macOS, Linux, and Windows** alike. It's idempotent: an
entry that already exists (ours, or one you added) means no write and no prompt.
That's for the server machine only; other devices still resolve `care.local` via
the mDNS responder above. No machine is ever renamed.

This needs administrator rights, and so does trusting the CA below, so the two are
batched into **one approval**, see [below](#one-install-one-approval).

---

## Why the frontend is built locally

The frontend (`care_fe`) is a **Vite** app: it **bakes** `REACT_CARE_API_URL` into
the static files at *build* time, and its build validator rejects an empty value.
The official prebuilt image points at CARE's cloud API — useless for an offline
clinic. So setup **builds the frontend once**, pinned to `https://care.local`. The
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

On Linux the config dir is `~/.config/care-desktop/`; on Windows `config.json` is in
`%AppData%\care-desktop\`.

> **Windows kit location.** On Windows the **kit** is staged under the home dir
> (`%USERPROFILE%\care-desktop\kit`), *not* `%AppData%`. Docker Desktop's WSL2 file
> share can't reliably read files created under `%AppData%` (both Roaming and Local) —
> the bind-mounted `clinic_settings.py`/`Caddyfile` then arrive in the container as
> empty directories and the backend fails with `ModuleNotFoundError`. The home dir is
> read live, so the kit lives there; only `config.json` (which Docker never reads)
> stays in `%AppData%`.

> **Patient data lives in the Docker volumes**, not in the install folder. The
> install folder only holds the kit + source clones. This is why moving/clearing the
> install folder never loses data — but you must still back up the volumes (the
> `backup` container does this automatically).
