# CARE Desktop

Self-contained, offline [CARE](https://github.com/ohcnetwork/care) for a small clinic.
One installer on one computer runs the whole EMR — backend, web app, database, file
storage and nightly encrypted backups — and staff open it from any phone or laptop
on the clinic Wi-Fi at `https://<clinic>.local`. No cloud account, and no internet
needed after the first install.

**Download:** [latest release](https://github.com/ohcnetwork/care_desktop/releases/latest)
— `.dmg` for macOS (Apple Silicon and Intel), `-setup.exe` for Windows 64-bit.

**Previously hosted CARE on this computer, but now cannot open another clinic?**
See [client recovery and earlier-install cleanup](docs/client-recovery.md).

## What it does

- **Setup wizard** — checks the computer (Docker, Git, network), takes a backup
  folder and password and an admin login, then builds and starts CARE.
- **Control panel** — start/stop/restart, start at login, the clinic address, and
  an "Add a device" page that trusts the local certificate on staff devices.
- **Backups** — nightly encrypted backups of the database and uploaded files,
  back up now, restore from the backup folder or an imported file.
- **Advanced** — plain-language clinic settings (backups, sign-in, patient SMS
  codes, email, branding, languages, visits, billing), backend plugins, log, uninstall.

## How it works

A Go / [Wails](https://wails.io) desktop app with a React UI drives a Docker Compose
stack: the CARE backend and workers, the CARE frontend, PostgreSQL, Redis, Silo for
files, and Caddy with the Coraza WAF as the HTTPS front door. Both CARE images are
built on the clinic's machine from the upstream commits pinned in `deployments/.env`.

| Path | What lives there |
|---|---|
| `app/` | The desktop app: Go engine in `internal/`, Wails bindings in `*.go`, React UI in `frontend/` |
| `deployments/` | The kit installed on the clinic computer: compose file, Caddyfile, env files, backup script, device-setup page |
| `.github/workflows/` | CI, and the release that builds the installers when a `vX.Y.Z` tag is pushed |

**Backend documentation:** [Start with `docs/README.md`](docs/README.md) for the
architecture, file map, Wails API, configuration, lifecycle, backups, native
integrations and release workflow.

Build with `cd app && wails build` (needs Go, Node 22 and the Wails CLI). MIT
licensed — see [LICENSE](LICENSE). Part of the [Open Healthcare Network](https://ohc.network).
