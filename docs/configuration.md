# Configuration — every setting explained

All clinic settings live in three files at the repo root. The desktop app's
**Settings** section edits the first two; you can also edit the files directly.

| File | Applied by | When it takes effect |
|---|---|---|
| [`backend.env`](#backendenv) | `care start` | container recreated, re-reads the file — **no image rebuild** |
| [`frontend.env`](#frontendenv) | `care rebuild-frontend` | **image rebuilt** (Vite bakes values at build time) |
| [`versions.env`](#versionsenv) | `care setup` / rebuild | controls which versions are built |

> **Key difference:** backend settings are read at container start, so changing them
> is cheap. Frontend settings are *frozen into the JavaScript at build time*, so
> changing them requires rebuilding the frontend image (a few minutes).

There are also [engine variables](#engine-variables) set by the app/CLI (not stored
in a file) — e.g. the backup folder and the admin password.

---

## `backend.env`

The single source of truth for the **backend + both celery services**. Edit a
value, then `care start`.

### Django settings module
| Variable | Default | Meaning |
|---|---|---|
| `DJANGO_SETTINGS_MODULE` | `clinic_settings` | Selects the clinic settings for a self-signed LAN cert (see [architecture](architecture.md#https-on-the-lan-clinic_settingspy)). **Don't change.** |
| `PYTHONPATH` | `/settings:/app` | Lets Python find `clinic_settings.py` (mounted at `/settings`). **Don't change.** |

### Database (PostgreSQL)
| Variable | Default | Meaning |
|---|---|---|
| `POSTGRES_USER` | `postgres` | DB username. |
| `POSTGRES_PASSWORD` | `postgres` | DB password. Fine on an isolated LAN box; change it if the server is shared. |
| `POSTGRES_HOST` | `db` | The Docker service name — **leave as `db`** (containers talk by service name). |
| `POSTGRES_DB` | `care` | Database name. |
| `POSTGRES_PORT` | `5432` | DB port (internal to Docker). |
| `DATABASE_URL` | `postgres://postgres:postgres@db:5432/care` | Full connection string. **Must match the four values above.** |

> If you change the password, update **both** `POSTGRES_PASSWORD` and `DATABASE_URL`.

### Redis (cache + Celery)
| Variable | Default | Meaning |
|---|---|---|
| `REDIS_URL` | `redis://redis:6379/0` | Cache backend. Leave as-is. |
| `CELERY_BROKER_URL` | `redis://redis:6379/0` | Background-job queue. Leave as-is. |

### Django core
| Variable | Default | Meaning |
|---|---|---|
| `DJANGO_SECRET_KEY` | *auto-generated* | Cryptographic key. **`care setup` replaces the `CHANGE_ME` placeholder with a random key on first run.** Keep it secret; changing it logs everyone out. |
| `DJANGO_DEBUG` | `False` | Never enable on a box holding patient data. |
| `DJANGO_ALLOWED_HOSTS` | `["*"]` | Which hostnames the backend answers to. `*` is fine on a private LAN. |
| `DJANGO_ADMIN_URL` | `admin` | Path of the Django admin (`/admin`). |
| `DJANGO_SECURE_SSL_REDIRECT` | `False` | Keep `False` — **Caddy** already redirects `http://` → `https://`, so Django must not redirect again (it would loop, since Caddy forwards to the backend over http internally). The `X-Forwarded-Proto: https` header still tells Django the original request was secure. |
| `DJANGO_SECURE_HSTS_PRELOAD` | `False` | Off — a self-signed LAN cert must not HSTS-pin clients. |
| `DJANGO_SECURE_HSTS_INCLUDE_SUBDOMAINS` | `False` | Off — self-signed LAN cert; no HSTS. |
| `DJANGO_SECURE_CONTENT_TYPE_NOSNIFF` | `False` | Off for LAN. |
| `CSRF_TRUSTED_ORIGINS` | `["https://care.local"]` | Origins allowed to POST to `/admin`. **Add your server IP origin** here if you access the admin by IP, e.g. `["https://care.local","https://192.168.1.50"]`. |

### Object storage (MinIO)
File uploads/downloads use **presigned URLs** — the browser talks to MinIO
*directly*, so the endpoint must be reachable from **every device**.

| Variable | Default | Meaning |
|---|---|---|
| `BUCKET_EXTERNAL_ENDPOINT` | `https://care.local` | The URL devices use to reach files (served through Caddy on the same origin as the app). **Never `localhost`** (that means *their* device). Use `https://<server-ip>` if devices can't resolve `care.local`. |
| `BUCKET_ENDPOINT` | `http://minio:9000` | Internal endpoint the backend uses (inside the Docker network, plain http). Leave as-is. |
| `BUCKET_REGION` | `ap-south-1` | S3 region label (any valid value). |
| `BUCKET_KEY` | `minioadmin` | Access key. Change for a non-trivial deployment (keep equal to `MINIO_ACCESS_KEY`). |
| `BUCKET_SECRET` | `minioadmin` | Secret key (keep equal to `MINIO_SECRET_KEY`). |
| `FILE_UPLOAD_BUCKET` | `patient-bucket` | Bucket for patient files (private). |
| `FACILITY_S3_BUCKET` | `facility-bucket` | Bucket for public assets (logos). |
| `MINIO_ACCESS_KEY` | `minioadmin` | MinIO root user — **keep equal to `BUCKET_KEY`.** |
| `MINIO_SECRET_KEY` | `minioadmin` | MinIO root password — **keep equal to `BUCKET_SECRET`.** |

> To change MinIO credentials you must update all four (`BUCKET_KEY`, `BUCKET_SECRET`,
> `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`) **and** recreate the MinIO volume, since the
> root credentials are baked in on first run.

### Offline-safe placeholders
The production settings expect these to exist; on an offline LAN they're unused.
Leave them as dummy values.

| Variable | Default | Meaning |
|---|---|---|
| `SNS_ACCESS_KEY` / `SNS_SECRET_KEY` | `123` | AWS SNS (SMS) — disabled offline. |
| `EMAIL_HOST` / `EMAIL_USER` / `EMAIL_PASSWORD` | `123` | Email sending — disabled offline. |

> **Consequence:** SMS/email OTP and notifications don't work offline. Use
> **password (+ authenticator-app TOTP)** login, which is fully local.

### Backups
| Variable | Default | Meaning |
|---|---|---|
| `DB_BACKUP_RETENTION_PERIOD` | `14` | Days of backups to keep; older ones are pruned. See [backups.md](backups.md). |

---

## `frontend.env`

Baked into the frontend image at **build** time. Edit, then `care rebuild-frontend`.

| Variable | Default | Meaning |
|---|---|---|
| `REACT_CARE_API_URL` | `https://care.local` | Backend base URL **without** `/api`. Must be a valid URL (empty is rejected by the build). Keeping it the same host as the app makes it same-origin (no CORS). Changing it needs `care rebuild-frontend` (Vite bakes it in at build time). |
| `REACT_ALLOWED_LOCALES` | *(commented)* | Optional. Comma-separated languages, e.g. `"en,hi,ta,ml,mr,kn"`. |
| `REACT_DEFAULT_COUNTRY` | *(commented)* | Optional default country. |

> These **override** `care_fe`'s own committed `.env` (logos, locales, etc.) via a
> gitignored `.env.local` the build writes. You usually only need the API URL.

> **Changing the API host (e.g. to a static IP):** set `REACT_CARE_API_URL` to that
> host and run `care rebuild-frontend`. A device must be able to reach that host, or
> file previews/API calls fail.

---

## `versions.env`

Controls **which versions** of the backend and frontend are built. The engine reads
it; `docker-compose.yml` reads the exported image tags.

Currently a TODO placeholder — the engine clones `ohcnetwork/care` and
`ohcnetwork/care_fe` at branch **`develop`** and builds them locally as
`care:clinic` and `care_fe:clinic`.

To **pin a reproducible release**, set any of:

| Variable | Default | Meaning |
|---|---|---|
| `BACKEND_IMAGE` | `care:clinic` | The built backend image tag. |
| `FRONTEND_IMAGE` | `care_fe:clinic` | The built frontend image tag. |
| `CARE_BE_REF` | `develop` | Git ref (branch/tag) of `ohcnetwork/care` to build. |
| `CARE_FE_REF` | `develop` | Git ref of `ohcnetwork/care_fe` to build. |
| `CARE_BE_REPO` | `https://github.com/ohcnetwork/care.git` | Backend source repo. |
| `CARE_FE_REPO` | `https://github.com/ohcnetwork/care_fe.git` | Frontend source repo. |

Example — pin to a known-good week:
```ini
CARE_BE_REF=v25.1.0
CARE_FE_REF=v25.1.0
```
…then re-run setup (or `rebuild-backend` / `rebuild-frontend`) to rebuild at that ref.

---

## Clinic address

The address staff type in their browser, `https://care.local` by default. Set it in the
installer's **Clinic address** field (step 1), or with `CARE_MDNS_NAME` for the CLI.
Enter the label only; `.local` is added for you.

Change it when a second CARE clinic already runs on the same WiFi. Two installs
advertising the same name clash over mDNS, so give one of them its own, for example
`care.local` and `caretest.local`.

`care setup` rewrites the address everywhere it appears, in one pass:

| File | What changes |
|---|---|
| `Caddyfile` | the site address, and the cert-download referer check |
| `backend.env` | `BUCKET_EXTERNAL_ENDPOINT`, `CSRF_TRUSTED_ORIGINS` |
| `frontend.env` | `REACT_CARE_API_URL` |
| `setup/index.html` | the addresses shown on the cert-trust page |

The rewrite matches whatever host is currently in those files, not a fixed
`care.local`, so renaming a second time works too. Names are lowercase letters,
numbers and hyphens.

> The frontend **bakes** its API URL at build time, so changing the address after
> setup needs `care rebuild-frontend` (or **Save & rebuild** in the app). Devices that
> trusted the old certificate keep working: the CA is unchanged, but they must use the
> new address.

> Two clinics on **one computer** is a different problem: they would collide on ports
> 80/443 and on the `care-desktop` compose project name. This setting is for two
> clinics on one **network**.

---

## Engine variables

Not stored in a file — set by the desktop app (from the wizard) or as environment
variables when using the CLI.

| Variable | Set by | Meaning |
|---|---|---|
| `BACKUP_DIR` | installer "Backup location" | Where daily backups go. Default `~/Desktop/care-db-backups`. |
| `CARE_ADMIN_PASSWORD` | installer "Admin password" | Password for the first `admin` user. Default `admin`. |
| `CARE_MDNS_NAME` | installer "Clinic address" | The host label, without `.local`. See [Clinic address](#clinic-address). |
| `CARE_NO_MDNS` | `1` from the GUI | Skip the engine's own hostname-rename (the GUI verifies `care.local` as a step instead). |
| `CARE_DESKTOP_DIR` | CLI override | Point the CLI at a specific kit folder (default: current directory). |
| `CARE_BE_DIR` / `CARE_FE_DIR` | advanced | Where the source clones live (default `<kit>/care`, `<kit>/care_fe`). |

---

## After changing a setting — what to run

| You changed… | Run |
|---|---|
| Any value in `backend.env` | `care start` (or **Save & apply** in the app) |
| Any value in `frontend.env` | `care rebuild-frontend` (or **Save & rebuild** in the app) |
| A version in `versions.env` | `care rebuild-backend` and/or `care rebuild-frontend` |

Nothing else needs editing — no files inside the images, no core CARE code.
