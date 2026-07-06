# `app/` — the CARE Desktop control app (Go / Wails)

One Go codebase that drives the whole clinic stack on macOS, Linux, and Windows —
**no shell, no Rust**. It ships as a desktop app **and** a `care` CLI sharing one
engine (`internal/care/`).

```
main.go            Wails entry — embeds frontend/dist + kit
app.go             JS-facing bridge (window.go.main.App.*)
autostart.go       launch-at-login per OS
internal/care/     the engine (pure Go; drives docker/git; no Wails import)
cmd/care/          the CLI
frontend/          web UI (TS + Vite); scripts/stage-kit.mjs stages the kit
```

## Build / run
```bash
wails dev      # hot-reload dev (needs a display)
wails build    # → build/bin/CARE Desktop(.app/.exe/binary)
go test ./internal/care/   # engine tests
```

## Full docs
- **How it works:** [`../docs/architecture.md`](../docs/architecture.md)
- **Building, testing, releases:** [`../docs/building.md`](../docs/building.md)
- **The CLI:** [`../docs/cli.md`](../docs/cli.md)
- **Settings:** [`../docs/configuration.md`](../docs/configuration.md)
