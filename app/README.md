# `app/` — the CARE Desktop control app (Go / Wails)

One Go codebase that drives the whole clinic stack on macOS, Linux, and Windows —
**no shell, no Rust**. The Wails GUI is a thin binding layer over the engine
packages under `internal/`, which never import Wails.

```bash
wails dev      # hot-reload dev (needs a display)
wails build    # → build/bin/CARE Desktop(.app/.exe/binary)
```

## Full docs
- **How it works:** [`../docs/architecture.md`](../docs/architecture.md)
- **Layout, building, releases:** [`../docs/building.md`](../docs/building.md)
- **Settings:** [`../docs/configuration.md`](../docs/configuration.md)
- **What must not change:** [`../docs/behaviour-contract.md`](../docs/behaviour-contract.md)
