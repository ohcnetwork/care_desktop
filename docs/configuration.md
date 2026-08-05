# Configuration — every setting explained

All clinic settings live in four files at the repo root. The desktop app's
**Settings** section edits them; you can also edit the files directly.

| File | Applied by | When it takes effect |
|---|---|---|
| [`backend.env`](#backendenv) | `care start` | container recreated, re-reads the file — **no image rebuild** |
| [`frontend.env`](#frontendenv) | `care rebuild-frontend` | **image rebuilt** (Vite bakes values at build time) |
| [`versions.env`](#versionsenv) | `care setup` / rebuild | controls which versions are built |
| [`tls.env`](#tlsenv) | `care start` | the clinic's web address; rebuilds the frontend if it changed |

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
| `DJANGO_SETTINGS_MODULE` | `clinic_settings` | Selects the clinic settings for running behind Caddy (see [architecture](architecture.md#django-behind-the-proxy-clinic_settingspy)). **Don't change.** |
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
| `DJANGO_SECURE_SSL_REDIRECT` | `False` | **Inert** — `clinic_settings.py` overrides all four of these. Off there because Django never sees an http request; Caddy is the only listener and is HTTPS-only. |
| `DJANGO_SECURE_HSTS_PRELOAD` | `False` | **Inert.** HSTS comes from Caddy. Preload is a one-way door — removal takes months — and is a poor fit for a clinic that may change domain. |
| `DJANGO_SECURE_HSTS_INCLUDE_SUBDOMAINS` | `False` | **Inert.** Would also force HTTPS on every subdomain of the clinic's domain, including unrelated ones. |
| `DJANGO_SECURE_CONTENT_TYPE_NOSNIFF` | `True` | **Inert**, but the behaviour is on: `clinic_settings.py` sets it and Caddy sends the header at the edge. |
| `CSRF_TRUSTED_ORIGINS` | placeholder | Origins allowed to POST to `/admin`. **Overridden at run time** from `CARE_PUBLIC_HOST` — edit [`tls.env`](#tlsenv), not this. |

### Object storage (MinIO)
File uploads/downloads use **presigned URLs** — the browser talks to MinIO
*directly*, so the endpoint must be reachable from **every device**.

| Variable | Default | Meaning |
|---|---|---|
| `BUCKET_EXTERNAL_ENDPOINT` | placeholder | The URL devices use to reach files (served through Caddy on :443, same origin as the app). **Overridden at run time** from `CARE_PUBLIC_HOST` — edit [`tls.env`](#tlsenv), not this. |
| `BUCKET_ENDPOINT` | `http://minio:9000` | Internal endpoint the backend uses. Leave as-is. |
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
| `REACT_CARE_API_URL` | placeholder | Backend base URL **without** `/api`. **Overwritten at every build** with the clinic's address from `CARE_PUBLIC_HOST`, so it can never drift from what Caddy serves. |
| `REACT_ALLOWED_LOCALES` | *(commented)* | Optional. Comma-separated languages, e.g. `"en,hi,ta,ml,mr,kn"`. |
| `REACT_DEFAULT_COUNTRY` | *(commented)* | Optional default country. |

> These **override** `care_fe`'s own committed `.env` (logos, locales, etc.) via a
> gitignored `.env.local` the build writes. You usually only need the API URL.

> **Don't set the API URL here.** The engine overwrites it from `CARE_PUBLIC_HOST` at
> every build, so the app can never end up calling an address Caddy doesn't serve. To
> change the clinic's address, edit [`tls.env`](#tlsenv) and run `care start`.

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

## `tls.env`

The clinic's web address. CARE is served over HTTPS on a domain you own, with a
publicly trusted certificate the server obtains and renews by itself; devices install
nothing. **This is required** — there is no plain-HTTP mode, and `care start` refuses
to run without it.

The real `tls.env` is gitignored (it holds a live credential) and written `0600`;
`tls.env.example` is the tracked template.

| Variable | Meaning |
|---|---|
| `CARE_PUBLIC_HOST` | The public name to serve as, e.g. `clinic.example.com`. An `A` record for it must point at this computer's LAN IP, proxy off. |
| `CLOUDFLARE_API_TOKEN` | Cloudflare token with `Zone:DNS:Edit` on that domain's zone. Used to answer the ACME DNS-01 challenge — the only method that works from behind clinic NAT. |
| `CARE_ACME_CA` | Certificate authority directory. Point at Let's Encrypt **staging** while testing; production allows only 5 identical certs per week. |
| `CARE_HSTS_SECONDS` | How long browsers remember to use HTTPS only for this clinic. Default `2592000` (30 days); `0` disables. See [tls.md](tls.md#whats-hardened-and-why). |
| `CARE_LAN_IP` | Which of this computer's addresses clients use. Leave empty unless the machine is on **two** networks — typically because it is itself the WiFi hotspot (a Pi access point at `192.168.4.1`). See [tls.md](tls.md#when-the-server-is-on-two-networks). |

Changing `CARE_PUBLIC_HOST` changes the origin the whole stack is built and signed
against, so `care start` also rebuilds the frontend (Vite bakes the API URL in) and
updates the presigned-file and CSRF origins for Django.

**Full walkthrough, including the Cloudflare setup: [tls.md](tls.md).**

---

## Engine variables

Not stored in a file — set by the desktop app (from the wizard) or as environment
variables when using the CLI.

| Variable | Set by | Meaning |
|---|---|---|
| `BACKUP_DIR` | installer "Backup location" | Where daily backups go. Default `~/Desktop/care-db-backups`. |
| `CARE_ADMIN_PASSWORD` | installer "Admin password" | Password for the first `admin` user. Default `admin`. |
| `CARE_DESKTOP_DIR` | CLI override | Point the CLI at a specific kit folder (default: current directory). |
| `CARE_BE_DIR` / `CARE_FE_DIR` | advanced | Where the source clones live (default `<kit>/care`, `<kit>/care_fe`). |

---

## After changing a setting — what to run

| You changed… | Run |
|---|---|
| Any value in `backend.env` | `care start` (or **Save & apply** in the app) |
| Any value in `frontend.env` | `care rebuild-frontend` (or **Save & rebuild** in the app) |
| A version in `versions.env` | `care rebuild-backend` and/or `care rebuild-frontend` |
| Anything in `tls.env` | `care start` (or **Save & apply HTTPS** in the app) |

Nothing else needs editing — no files inside the images, no core CARE code.
