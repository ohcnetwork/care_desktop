# CARE Desktop

**Run the whole [CARE](https://github.com/ohcnetwork/care) stack on one computer for
a small clinic — offline, on the local WiFi, at `http://care.local`.**

Backend + frontend + database + file storage, all on a single server, reachable by
every phone/laptop on the same WiFi. No internet after setup, no cloud, no accounts.
Drive it with a small **desktop app** (a few clicks) or the **`care` CLI**.

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

1. **Install Docker Desktop + Git** on the server computer, and start Docker. *(Windows also needs Apple Bonjour or a static IP — see the Windows guide.)*
2. **Download the app** for your OS from the **Releases** page (or build it — see [building.md](docs/building.md)).
3. **Open it** and complete the wizard: the three checks (Docker, Git, `care.local`) must be green, then click **Install & Start**.
4. **Open `http://care.local/`** on any device on the WiFi → log in **`admin` / `admin`** → change the password.

> First setup downloads + builds CARE (~10–20 min, needs internet **once**). After
> that it runs fully offline.

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
| `clinic_settings.py` | plain-HTTP Django settings for the LAN |
| `Caddyfile` | the reverse proxy (one address for app + API) |
| `minio/`, `scripts/` | MinIO bucket setup + the daily backup loop (run inside containers) |
| `docs/` | all documentation |

The repo-root files are the **single source of truth**; the app embeds them at build
time. The core CARE app is **never modified**.

---

## Highlights

- **Offline-first** — everything stays in the building; no internet needed to use it.
- **No-terminal option** — the desktop app installs + runs CARE with a few clicks.
- **Cross-platform** — one Go binary per OS; Windows needs no WSL or bash.
- **Data-safe** — daily DB + file backups with **one-click restore**; the app never deletes your volumes.
- **No core changes** — runs CARE's own images/source, configured from the outside.
