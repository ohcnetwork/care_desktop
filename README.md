# CARE Desktop

**Run the whole [CARE](https://github.com/ohcnetwork/care) stack on one computer for
a small clinic — on the local WiFi, over HTTPS, at your own domain.**

Backend + frontend + database + file storage, all on a single server, reachable by
every phone/laptop on the same WiFi. Traffic never leaves the building; no cloud, no
accounts. Drive it with a small **desktop app** (a few clicks) or the **`care` CLI**.

It's one Go program — no shell scripts, no Rust — that runs on **macOS, Linux, and
Windows**.

---

## 📖 Documentation

Full docs are in **[`docs/`](docs/README.md)**:

| | |
|---|---|
| 🍎 [Install on macOS](docs/install-macos.md) | 🪟 [Install on Windows](docs/install-windows.md) |
| 🐧 [Install on Linux](docs/install-linux.md) | 🔒 [**HTTPS & your clinic's address**](docs/tls.md) |
| ⚙️ [Configuration — every env var](docs/configuration.md) | 🏗️ [Architecture — how it works](docs/architecture.md) |
| 💻 [The `care` CLI](docs/cli.md) | 💾 [Backups & restore](docs/backups.md) |
| 🔧 [Troubleshooting](docs/troubleshooting.md) | 🛠️ [Building from source](docs/building.md) |

---

## Quick start

1. **Install Docker + Git** on the server computer, and start Docker. Any Docker engine works — Docker Engine, Colima, Podman, or Docker Desktop.
2. **Set up the clinic's web address** — a domain you own, its DNS on Cloudflare, an `A` record pointing at the server, and an API token. One-time, ~15 minutes: **[tls.md](docs/tls.md)**.
3. **Download the app** for your OS from the **Releases** page (or build it — see [building.md](docs/building.md)).
4. **Open it** and complete the wizard: Docker/Git and your clinic web address must be green, then click **Install & Start**.
5. **Open `https://clinic.yourdomain.com/`** on any device on the WiFi → log in **`admin` / `admin`** → change the password.

> First setup downloads + builds CARE (~10–20 min, needs internet). Afterwards the
> server only goes online to renew its certificate, roughly every two months.

Prefer a terminal? See the [CLI guide](docs/cli.md):
```bash
cd care-desktop && care setup && care start
```

---

## How the HTTPS works

Every device gets a real padlock with **nothing installed on it** — no certificates to
add to phones, no warning screens to click through.

The clinic's domain is public, but it resolves to the server's **private** address on
the clinic WiFi. So the name and the certificate paperwork are the only things that
touch the internet; patient traffic never leaves the building. Because the server sits
behind the router's NAT with no port forwarding, Let's Encrypt can't connect in to
verify it — instead Caddy proves domain ownership by writing a DNS record through the
Cloudflare API, which needs only outbound internet.

Only port **443** is published. There is no port 80: a plaintext entry point can be
intercepted and replaced with a fake login page, which would undo the encryption it
exists to hand off to.

**One limit worth knowing up front:** client devices need to *look up* the name, and
that lookup goes to the internet. If the clinic's connection drops, phones can't
resolve the address even though the server is metres away. A single static DNS entry
on the clinic router fixes this permanently — see
[tls.md](docs/tls.md#moving-networks-and-changing-addresses).

---

## What's in this repo

| Path | What |
|---|---|
| `app/` | the Go app — Wails desktop GUI + the `care` engine/CLI |
| `docker-compose.yml` | the stack: db, redis, minio, backend, celery×2, frontend, caddy, backup |
| `backend.env` / `frontend.env` | clinic settings ([reference](docs/configuration.md)) |
| `tls.env.example` | template for the clinic's domain + DNS token ([guide](docs/tls.md)) |
| `versions.env` | which CARE versions to build |
| `clinic_settings.py` | Django settings for running behind the reverse proxy |
| `Caddyfile` | the reverse proxy — one HTTPS address for app + API |
| `minio/`, `scripts/` | MinIO bucket setup + the daily backup loop (run inside containers) |
| `docs/` | all documentation |

The repo-root files are the **single source of truth**; the app embeds them at build
time. The core CARE app is **never modified**.

> `tls.env` holds a live API token, so it is gitignored and written `0600`. Builds
> always stage the blank `tls.env.example`, never your own.

---

## Highlights

- **Encrypted end to end** — real HTTPS on your own domain; devices install nothing.
- **Stays in the building** — patient data never crosses the internet.
- **Self-maintaining** — the server renews its own certificate and keeps its own DNS
  record current when the network changes.
- **No-terminal option** — the desktop app installs + runs CARE with a few clicks.
- **Cross-platform** — one Go binary per OS; Windows needs no WSL or bash.
- **Data-safe** — daily DB + file backups with **one-click restore**; the app never deletes your volumes.
- **No core changes** — runs CARE's own images/source, configured from the outside.
