# Building from source (developers)

CARE Desktop's control app is a [Wails](https://wails.io) (Go + web) project in
`app/`. One codebase produces the desktop app **and** the `care` CLI, sharing the
engine in `app/internal/care/`.

For *what the code does*, see [architecture.md](architecture.md). This page is about
*building* it.

---

## Toolchain

| Tool | Version | Notes |
|---|---|---|
| **Go** | 1.23+ | the engine, CLI, and Wails backend |
| **Node** | 20+ | builds the web UI (Vite) |
| **Wails CLI** | v2 | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |
| **C toolchain** | — | macOS: Xcode CLT · Linux: `build-essential` + WebKitGTK dev libs · Windows: MSVC build tools |

Linux desktop build deps:
```bash
sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev build-essential
```

---

## Layout

```
app/
  main.go            Wails entry — embeds frontend/dist + kit
  app.go             JS-facing bridge (window.go.main.App.*)
  autostart.go       launch-at-login per OS
  internal/care/     the engine (pure Go; no Wails import)
  cmd/care/          the CLI
  frontend/          web UI (TS + Vite); scripts/stage-kit.mjs stages the kit
```

The repo-root files (`docker-compose.yml`, `*.env`, `Caddyfile`, `minio/`,
`scripts/`) are the **kit** — the single source of truth. The frontend build copies
them into `app/kit/` (gitignored) so Go can `//go:embed` them. Don't edit `app/kit/`.

---

## Run in development
```bash
cd app
wails dev
```
Opens the app with hot-reload: edit `frontend/src/*` (instant) or any `.go` file
(recompiles). Needs a display. The browser devtools view is at `http://localhost:34115`.

---

## Build a release binary
```bash
cd app
wails build                       # current OS → build/bin/CARE Desktop(.app/.exe/binary)
wails build -platform darwin/universal   # mac universal (Intel + Apple Silicon)
```
`wails build` runs the frontend build first (which stages the kit), then compiles Go
and packages the app. The kit ends up embedded in the binary.

---

## Build just the CLI
```bash
cd app
go build -o care ./cmd/care
```
No Wails/Node needed for the CLI — it's pure Go + stdlib.

---

## Tests
```bash
cd app
go test ./internal/care/
```
Covers the secret generator (idempotent, strong), the config-resolution ladder, and
real `docker compose` wiring (brings up db+redis, tears them down; skips if Docker is
absent). See [architecture.md](architecture.md) for the engine design.

```bash
go vet ./...        # static checks
```

---

## Releases (CI)

`.github/workflows/release.yml` builds all three OSes on a `v*` tag:

1. Push a tag: `git tag v0.1.0 && git push --tags`.
2. The matrix (macOS / Ubuntu / Windows) runs `wails build`, zips/tars the artifact,
   and attaches it to a **draft GitHub Release**.
3. Review the draft and publish.

Artifacts: `CARE-Desktop-macos.zip` (`.app`), `CARE-Desktop-windows.zip` (`.exe`),
`CARE-Desktop-linux.tar.gz` (binary).

> **Signing/notarization** isn't set up yet, so users see an "unsigned app" warning
> on first open (right-click → Open on macOS; "More info → Run anyway" on Windows).
> Add Apple/Windows signing secrets to the workflow to remove it.

---

## Pinning CARE versions

By default the engine builds `ohcnetwork/care` and `ohcnetwork/care_fe` at branch
`develop`. To ship a reproducible release, set `CARE_BE_REF` / `CARE_FE_REF` (and the
image tags) in `versions.env` — see [configuration.md](configuration.md#versionsenv).
