# Configuration — every setting explained

All clinic settings live in three files in [`deployments/`](../deployments). The
desktop app's **Settings** section edits the first two; you can also edit the files
directly. On an installed clinic the same files live in the app's install dir.

| File | Applied by | When it takes effect |
|---|---|---|
| [`backend.env`](#backendenv) | **Save & apply** | container recreated, re-reads the file — **no image rebuild** |
| [`frontend.env`](#frontendenv) | `care rebuild-frontend` | **image rebuilt** (Vite bakes values at build time) |
| [`.env`](#env) | Install & Start / rebuild | release pins: which images + refs are used |

> **Key difference:** backend settings are read at container start, so changing them
> is cheap. Frontend settings are *frozen into the JavaScript at build time*, so
> changing them requires rebuilding the frontend image (a few minutes).

There are also [engine variables](#engine-variables) set by the app/CLI (not stored
in a file) — e.g. the backup folder and the admin password.

---

## `backend.env`

The single source of truth for the **backend + both celery services**. Edit a
value, then **Save & apply**.

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
| `DJANGO_SECRET_KEY` | *auto-generated* | Cryptographic key. **Setup replaces the `CHANGE_ME` placeholder with a random key on first run.** Keep it secret; changing it logs everyone out. |
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
The deployment settings expect these to exist; on an offline LAN they're unused.
Leave them as dummy values.

| Variable | Default | Meaning |
|---|---|---|
| `SNS_ACCESS_KEY` / `SNS_SECRET_KEY` | `123` | AWS SNS (SMS) — never reached; see `SMS_BACKEND` below. |
| `EMAIL_HOST` / `EMAIL_USER` / `EMAIL_PASSWORD` | `123` | Email sending — disabled offline. |

### SMS / OTP
| Variable | Default | Meaning |
|---|---|---|
| `SMS_BACKEND` | `care.utils.sms.backend.base.SmsBackendBase` | CARE's abstract base backend: sending raises immediately, with no network call. **Don't point this at a real backend unless you have an SMS gateway on the LAN.** |

> **Why a deliberately broken backend:** CARE only falls back to its hardcoded
> non-production OTP (`45612`) when it believes SMS is switched *off*. So the clinic
> sets `USE_SMS = True` in `clinic_settings.py` and hands it a backend that fails —
> the send errors out and **no OTP is ever stored**. Together these close patient OTP
> login and staff password-reset-by-phone, which on the LAN would otherwise let anyone
> who knows a phone number log in as that patient, or take over that staff account.
>
> **Consequence:** SMS/email OTP and notifications don't work offline — by design.
> Staff use **password (+ authenticator-app TOTP)** login, which is fully local, and
> patients are registered and booked by staff (the public portal is switched off in
> [`frontend.env`](#frontendenv)).

### Login rate limit
Not set by default — listed here because the defaults behave differently offline.

| Variable | Default | Meaning |
|---|---|---|
| `RATE_LIMIT` | `5/10m` | Wrong passwords per username before login returns `429`. |
| `DISABLE_RATELIMIT` | `False` | `True` switches the throttle off entirely. |

> A rate-limited login asks for a Google reCAPTCHA, which can't load offline, so the
> only way past a `429` is to wait out the window (10 minutes by default). The clinic
> keeps the throttle — it self-heals, and brute-force protection matters on a shared
> WiFi — but loosen `RATE_LIMIT` or set `DISABLE_RATELIMIT=True` if staff hit it in
> practice. The frontend is configured not to render the dead captcha box.

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
| `REACT_DISABLE_PATIENT_LOGIN` | `true` | Hides the public landing page and the patient OTP portal, so `https://care.local/` opens staff login. Keep it on: an offline clinic can't deliver an OTP, and patients are registered and booked by staff. |
| `REACT_RECAPTCHA_SITE_KEY` | *(empty)* | Empty on purpose — the login form shows a Google reCAPTCHA after a rate-limited login, and `google.com` is unreachable offline. Empty means the widget isn't rendered at all. |
| `REACT_ALLOWED_LOCALES` | *(commented)* | Optional. Comma-separated languages, e.g. `"en,hi,ta,ml,mr,kn"`. |
| `REACT_DEFAULT_COUNTRY` | *(commented)* | Optional default country. |

> These **override** `care_fe`'s own committed `.env` (logos, locales, etc.) via a
> gitignored `.env.local` the build writes. You usually only need the API URL — the
> other two are clinic hardening you shouldn't need to touch.

> **Turning the patient portal back on** takes more than flipping
> `REACT_DISABLE_PATIENT_LOGIN`: the OTP endpoints stay closed until the backend has a
> working `SMS_BACKEND` (see [above](#sms--otp)). Without one, the portal renders but
> no patient can get past the OTP screen.

> **Changing the API host (e.g. to a static IP):** set `REACT_CARE_API_URL` to that
> host and run `care rebuild-frontend`. A device must be able to reach that host, or
> file previews/API calls fail.

---

## `.env`

The **single source of truth** for every image and source ref the clinic runs.

Compose auto-loads this file from the project directory, and the app embeds a copy
so pins resolve before an install dir exists. There are deliberately **no fallback
values** anywhere else — not in `docker-compose.yml`, not in Go. A missing pin makes
the stack refuse to start with a message naming the variable, rather than quietly
running a different image. (That is not hypothetical: `BACKEND_IMAGE` previously had
three definitions, and Compose's fallback pointed at `ghcr.io/ohcnetwork/care:latest`
— an unpinned image from the internet — for anyone running `docker compose` by hand.)

Every value is **required**. The app loads this file once at startup, checks that
all twelve pins are present, and refuses to start otherwise — naming the missing
ones. Compose does the same with `${VAR:?}`. Both sides parse it with the same
library (`compose-spec/compose-go/dotenv`, the one Compose itself uses), so quoting
and escaping can never diverge between them.

Third-party images are tracked manually. Check upstream release notes before
bumping one — a Postgres major is a one-way door, because the `postgres-data`
volume is not forward compatible and there is no downgrade path.

**Clinic settings do not belong here.** Compose interpolates this file, so every
value in it is visible project-wide. Passwords and hostnames live in `backend.env` /
`frontend.env`, which are mounted into containers instead.

| Variable | Current | Meaning |
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

Setup rewrites the address everywhere it appears, in one pass:

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

## Operator choices

Not stored in a settings file and **not readable from the environment**. They are
typed fields on the engine (`clinic.Clinic`), set by the wizard for the run that
needs them, so a password can never arrive from a stray shell variable or from
`.env` (which Compose interpolates into every service). The two passwords are never
written to `config.json` either — only a bcrypt hash of the admin one, to gate
Advanced settings.

| Value | Chosen in | Meaning |
|---|---|---|
| Backup location | installer folder picker | Where daily backups go. Default `~/Desktop/care-db-backups`. |
| Admin password | installer "Admin password" | Password for the first `admin` user. **No default** — without it the superuser is not created. |
| Backup password | installer "Backup password" | Encrypts backups. Empty means backups are written in plaintext. |
| Clinic address | installer "Clinic address" | The host label, without `.local`. See [Clinic address](#clinic-address). |

`CARE_DESKTOP_DIR` is the one real environment variable left: it points the app at a
specific install dir instead of the default per-user one. Development only.

---

## After changing a setting — what to run

| You changed… | Run |
|---|---|
| Any value in `backend.env` | **Save & apply** in the app |
| Any value in `frontend.env` | **Save & rebuild** in the app |
| A version in `.env` | **Rebuild backend** / **Rebuild frontend** in Advanced settings |
| `clinic_settings.py` (bind-mounted, e.g. `USE_SMS`) | **Save & apply** — it's read at container start |

Nothing else needs editing — no files inside the images, no core CARE code.
