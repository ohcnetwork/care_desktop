# CARE Desktop — Documentation

CARE Desktop runs the entire [CARE](https://github.com/ohcnetwork/care) stack
(backend + frontend + database + file storage) on **one computer** for a small
clinic, reachable across the local WiFi at **http://care.local** — with **no
internet needed** after the one-time setup.

You run it either with a small **desktop app** (a few clicks, for non-technical
staff) or the **`care` command-line tool**. Both are one Go program; they share
the same engine and behave identically.

---

## Start here

| If you want to… | Read |
|---|---|
| Install on a **Mac** | [install-macos.md](install-macos.md) |
| Install on **Windows** | [install-windows.md](install-windows.md) |
| Install on **Linux** | [install-linux.md](install-linux.md) |
| Understand how it all fits together | [architecture.md](architecture.md) |
| Change a setting (every env variable explained) | [configuration.md](configuration.md) |
| Use the terminal commands | [cli.md](cli.md) |
| Back up / restore data | [backups.md](backups.md) |
| Fix a problem | [troubleshooting.md](troubleshooting.md) |
| Build the app from source (developers) | [building.md](building.md) |

---

## The 60-second overview

1. **One computer is the server.** It stays on and runs Docker. Everything lives
   here — the app, the database, the uploaded files.
2. **Setup builds CARE once.** The first run downloads + builds the backend and
   frontend images (needs internet *once*), then starts everything.
3. **Every other device just opens a web page.** Phones, tablets, and laptops on
   the same WiFi open `http://care.local` and log in. They install nothing.
4. **It's offline after that.** No cloud, no accounts, no internet — all data
   stays in the building.

```
  Phones / laptops / tablets on the clinic WiFi
                     │
            http://care.local
                     │
        ┌────────────▼─────────────┐   ONE server computer (Docker)
        │  Caddy (reverse proxy :80)│
        │     ├─ /api,/admin → backend (Django)
        │     └─ everything else → frontend (React)
        │  postgres · redis · minio (files)
        │  celery worker + beat · daily backup
        └───────────────────────────┘
```

## Requirements at a glance
- **Docker**, running — any engine (Docker Engine, Colima, Podman, Rancher Desktop, OrbStack, or Docker Desktop), with the `docker compose` v2 plugin. *Required on every OS.*
- **git** — used once to download + build CARE.
- A way to be found as `care.local`: automatic on macOS/Linux (the setup sets the
  hostname), **Apple Bonjour or a static IP** on Windows. See your OS install guide.

> First-time setup downloads several GB of Docker images and builds the frontend —
> budget **~10–20 minutes** and a working internet connection **for that step only**.
