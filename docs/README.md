# CARE Desktop — Documentation

CARE Desktop runs the entire [CARE](https://github.com/ohcnetwork/care) stack
(backend + frontend + database + file storage) on **one computer** for a small
clinic, reachable across the local WiFi at **https://your-own-domain** — with patient
data never leaving the building.

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
| Turn on **HTTPS** with your own domain | [tls.md](tls.md) |
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
   the same WiFi open `https://clinic.yourdomain.com` and log in. They install nothing —
   the padlock works out of the box.
4. **Data stays in the building.** The address is public but resolves to the server's
   address on the clinic WiFi, so traffic never crosses the internet. The server only
   goes online to renew its certificate, about every two months.

```
  Phones / laptops / tablets on the clinic WiFi
                     │
        https://clinic.yourdomain.com      (resolves to the server's LAN IP)
                     │
        ┌────────────▼──────────────┐  ONE server computer (Docker)
        │  Caddy (reverse proxy :443)│
        │     ├─ /api,/admin → backend (Django)
        │     └─ everything else → frontend (React)
        │  postgres · redis · minio (files)
        │  celery worker + beat · daily backup
        └───────────────────────────┘
```

## Requirements at a glance
- **Docker**, running — any engine (Docker Engine, Colima, Podman, Rancher Desktop, OrbStack, or Docker Desktop), with the `docker compose` v2 plugin. *Required on every OS.*
- **git** — used once to download + build CARE.
- **A domain**, with its DNS on Cloudflare and an `A` record pointing at the server's
  address on the clinic WiFi. See [tls.md](tls.md) — this is what gives every device a
  working padlock with nothing to install.
- **Internet on the server** — for setup, and afterwards only to renew the certificate
  (about every two months).

> First-time setup downloads several GB of Docker images and builds the frontend —
> budget **~10–20 minutes**.
