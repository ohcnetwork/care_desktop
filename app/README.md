# `app/` — the CARE Desktop control app (Go / Wails)

The Go/Wails application operates the clinic through Docker Compose and native
system tools. The engine packages under `internal/` never import Wails.

```bash
wails dev
```

For a production build, synchronize installer metadata before Wails starts:

```bash
node frontend/scripts/stage-install.mjs
wails build
```

See [Development and release](../docs/development-and-release.md) for preparing
a fresh checkout, embedded assets, build prerequisites, and platform packaging.
Running a development app can operate an existing clinic on this computer.
For publishing installers, follow [Releasing CARE Desktop](../docs/releases.md).

## Backend documentation

Start at the [documentation index](../docs/README.md), then read the
[architecture](../docs/architecture.md), [file map](../docs/repository-map.md),
[Wails API](../docs/wails-application.md), and
[configuration guide](../docs/configuration-and-settings.md).
