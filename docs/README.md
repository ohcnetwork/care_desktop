# CARE Desktop — Documentation

CARE Desktop runs the entire [CARE](https://github.com/ohcnetwork/care) stack
(backend + frontend + database + file storage) on **one computer** for a small
clinic, reachable across the local WiFi at **https://care.local** — with **no
internet needed** after the one-time setup.

You run it either with a small **desktop app** (a few clicks, for non-technical
staff) or the **`care` command-line tool**. Both are one Go program; they share
the same engine and behave identically.

Traffic is served over **HTTPS** with a self-signed certificate from a built-in local
CA. The server trusts its own cert automatically; every other device trusts it once
from the built-in **[`https://care.local/setup`](architecture.md#certificate-trust--the-setup-bootstrap)**
page (two taps). See [architecture.md](architecture.md#https-on-the-lan-clinic_settingspy) for why HTTPS
is required.

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
   the same WiFi open `https://care.local` and log in. They install nothing — but
   trust the clinic's certificate once from `https://care.local/setup` (two taps).
4. **It's offline after that.** No cloud, no accounts, no internet — all data
   stays in the building.

```
  Phones / laptops / tablets on the clinic WiFi
                     │
            https://care.local   (self-signed cert, trusted once via /setup)
                     │
        ┌────────────▼─────────────┐   ONE server computer (Docker)
        │  Caddy (reverse proxy)    │   :80 → redirect to https · :443 → the app
        │     ├─ /api,/admin → backend (Django)
        │     ├─ /setup,/root.crt → cert-trust bootstrap
        │     └─ everything else → frontend (React)
        │  postgres · redis · minio (files)
        │  celery worker + beat · daily backup
        └───────────────────────────┘
```

## Requirements at a glance
- **Docker**, running — any engine (Docker Engine, Colima, Podman, Rancher Desktop, OrbStack, or Docker Desktop), with the `docker compose` v2 plugin. *Required on every OS.*
- **git** — used once to download + build CARE.
- A way to be found as `care.local`: *other devices* are served by the in-app mDNS
  responder (automatic on macOS/Linux; on Windows recent Windows 11 resolves it
  natively, otherwise **Apple Bonjour or a static IP**). The **server itself** uses a
  hosts entry setup adds on every OS, with no rename. See your OS install guide.

> First-time setup downloads several GB of Docker images and builds the frontend —
> budget **~10–20 minutes** and a working internet connection **for that step only**.
