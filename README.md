# CARE Desktop

**Run the whole [CARE](https://github.com/ohcnetwork/care) stack on one computer for
a small clinic — offline, on the local WiFi, at `https://care.local`.**

Backend + frontend + database + file storage, all on a single server, reachable by
every phone/laptop on the same WiFi. No internet after setup, no cloud, no accounts.
Drive it with a small **desktop app** (a few clicks) or the **`care` CLI**.

Traffic is served over **HTTPS** with a self-signed certificate from a built-in local
CA (browsers need a real `https://` origin for the camera/scanner and secure cookies —
see [architecture.md](docs/architecture.md#https-on-the-lan)). The server trusts its
own cert automatically; other devices trust it in two taps from the built-in
**`https://care.local/setup`** page.

It's one Go program — no shell scripts, no Rust — that runs on **macOS, Linux, and
Windows**.

---

## 📖 Documentation

Full docs are in **[`docs/`](docs/README.md)**:

| | |
|---|---|
| 🍎 [Install on macOS](docs/install-macos.md) | 🪟 [Install on Windows](docs/install-windows.md) |
| 🐧 [Install on Linux](docs/install-linux.md) | ⚙️ [Configuration — every env var](docs/configuration.md) |
| 🏗️ [Architecture — how it works](docs/architecture.md) | 💻 [The `care` CLI](docs/cli.md) |
| 💾 [Backups & restore](docs/backups.md) | 🔧 [Troubleshooting](docs/troubleshooting.md) |
| 🛠️ [Building from source](docs/building.md) | |

---

## Quick start

1. **Install Docker + Git** on the server computer, and start Docker. Any Docker engine works — Docker Engine, Colima, Podman, or Docker Desktop. *(On Windows use Docker Desktop with WSL 2 integration — Docker installed only inside WSL isn't visible to the app. Other devices reaching a Windows server may need Bonjour or a static IP — see the Windows guide.)*
2. **Download the app** for your OS from the **Releases** page (or build it — see [building.md](docs/building.md)).
3. **Open it** and complete the wizard: the three checks (Docker, Git, `care.local`) must be green, then click **Install & Start**.
4. **Open `https://care.local/`** on any device on the WiFi → log in **`admin` / `admin`** → change the password.

> First setup downloads + builds CARE (~10–20 min, needs internet **once**). After
> that it runs fully offline.

### Trust the certificate (first visit on each device)

The site uses a self-signed cert, so a brand-new device shows a security warning until
it trusts the clinic's local CA **once**:

- **The server computer trusts itself automatically** the first time the stack starts —
  nothing to do there.
- **Every other device:** open **`https://care.local/setup`**, pick the device type, and
  follow the two-tap instructions (download `root.crt` → trust it). After that the
  padlock is green and the camera/file features work. On iOS this installs a
  configuration profile named **"CARE Desktop Local CA"**; Safari is required for the
  install.

> Uninstalling from the app also **removes the certificate from the server machine**.
> Other devices keep their copy — remove it from their trust store manually if needed.

Prefer a terminal? See the [CLI guide](docs/cli.md):
```bash
cd care-desktop && care setup && care start
```

---

## What's in this repo

| Path | What |
|---|---|
| `app/` | the Go app — Wails desktop GUI + the `care` engine/CLI |
| `docker-compose.yml` | the stack: db, redis, minio, backend, celery×2, frontend, caddy, backup |
| `backend.env` / `frontend.env` | all clinic settings ([reference](docs/configuration.md)) |
| `versions.env` | which CARE versions to build |
| `clinic_settings.py` | Django settings for the LAN — production settings served over `https://`, self-signed cert |
| `Caddyfile` | the reverse proxy: `https://care.local` (one origin), `:80` redirect, and the cert-trust bootstrap |
| `setup/` | the `https://care.local/setup` install page that hands the local CA to new devices |
| `load_test_fixtures.sh` | dev-only: seed the running backend with CARE's sample data ([see below](#loading-sample-data-for-testing)) |
| `minio/`, `scripts/` | MinIO bucket setup + the daily backup loop (run inside containers) |
| `docs/` | all documentation |

The repo-root files are the **single source of truth**; the app embeds them at build
time. The core CARE app is **never modified**.

---

## Loading sample data (for testing)

To try the clinic with realistic data, seed CARE's fixtures into the **running** stack:

```bash
./load_test_fixtures.sh
```

It installs `faker` into the backend container (ephemeral) and runs CARE's
`manage.py load_fixtures` under the deployment settings with `DEBUG=True`. The seeded
data lands in the postgres volume, so it **persists** across restarts (only the
ephemeral `faker` install is lost when the container is recreated). Start the stack
first (`care start` or the app), then run the script. See
[building.md](docs/building.md#loading-sample-data-dev) for details.

> This is a **development/demo** tool — don't seed fixtures onto a real clinic's data.

---

## Highlights

- **Offline-first** — everything stays in the building; no internet needed to use it.
- **HTTPS on the LAN** — a self-signed cert from a built-in CA; new devices trust it in two taps from `care.local/setup`, and the server trusts itself automatically.
- **No-terminal option** — the desktop app installs + runs CARE with a few clicks.
- **Cross-platform** — one Go binary per OS; Windows needs no WSL or bash.
- **Data-safe** — daily DB + file backups with **one-click restore**; the app never deletes your volumes.
- **No core changes** — runs CARE's own images/source, configured from the outside.
