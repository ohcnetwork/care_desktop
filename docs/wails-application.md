# Wails application and API

[Documentation index](README.md)

This guide covers `app/*.go` and the desktop-to-Go contract. Lower-level work is delegated to the [clinic engine](clinic-lifecycle.md), [backup store](backups-and-restore.md), and [native packages](native-integrations.md).

## Entry point and embedded resources

[`main.go`](../app/main.go) embeds two independent trees:

| Embedded filesystem | Source at compile time | Runtime use |
| --- | --- | --- |
| `assets` | `app/frontend/dist/` | Wails serves the desktop control panel from the executable. |
| `installFS` | `app/install/` | The application reads release pins and unpacks the deployment kit. |

The desktop UI is not served from the CARE frontend container. It must remain usable when the clinic stack is stopped or not installed.

`main()` opens the diagnostic log, installs its fatal-error callback, constructs `App`, records version/pin information, and calls `wails.Run`. Only the `App` instance is bound. Exported receiver methods become desktop-callable methods; helper functions and unexported methods do not.

The window title is `CARE Desktop`, with an initial size of 1180 by 900 and a minimum of 720 by 560. On macOS, `HideWindowOnClose` is enabled. The single-instance identifier is `ohc.care-desktop`; a second launch shows and unminimizes the existing window.

Startup failures are written to the log and stderr. `fatal()` attempts a native error dialog on macOS or Windows, then exits with status 1.

## Startup and shutdown

```mermaid
flowchart TD
    Main["main: open log"] --> New["NewApp"]
    New --> Path["Repair GUI process PATH"]
    Path --> Pins["Read embedded install/.env and validate release pins"]
    Pins --> Config["Resolve and load config.json"]
    Config --> Journal["Inspect pending restore metadata"]
    Journal --> Wails["wails.Run with bound App"]
    Wails --> Startup["startup: store Wails context"]
    Startup --> Refresh["Refresh installed kit when setup is complete and stable"]
    Refresh --> Advertise["Start mDNS advertiser and watcher"]
    Advertise --> Probe["Run initial Docker probe in a goroutine"]
    Probe --> UI["Desktop requests GetState"]
    UI --> First{"Setup complete and not removing?"}
    First -->|No| Wizard["Setup and residue flow"]
    First -->|Yes| Panel["Control panel"]
    Panel --> Start{"Unhealthy or restore pending?"}
    Start -->|Yes| Run["Request ClinicAction start"]
    Start -->|No| Observe["Observe running clinic"]
```

`NewApp` does not treat a missing embedded release file, malformed saved configuration, or invalid restore journal as an ordinary first run. It returns an error rather than allowing initialization against uncertain state.

[`refreshInstallDir()`](../app/app_lifecycle.go) runs under the mutation gate. It does nothing for an unconfigured or removing installation. It also leaves the installed kit untouched when a restore is pending. Otherwise it refreshes kit files and reapplies the saved clinic domain. A refresh error is logged; the callback does not silently rewrite the configuration as uninstalled.

The backend starts name advertising, but the desktop state store makes the normal "start CARE on launch" request. In [`care-store.tsx`](../app/frontend/src/state/care-store.tsx), `bootPanel()` requests Start if health is inactive **or** `restore_pending` is true.

### Advertiser lifetime

`App` owns the mDNS advertiser. Its watcher wakes every 30 seconds, restarts advertising when LAN addresses change, and retries after two consecutive response-check failures. It can also recreate an absent advertiser. No name is advertised when the name is empty or removal is in progress.

`shutdown()` closes the watcher's stop channel and stops the advertiser. Stopping the desktop's advertisement is distinct from stopping Docker containers.

### Closing the application

`beforeClose()` first attempts the exclusive operation lock. If work is active, closing is prevented. Otherwise it asks about a running clinic:

| Choice or condition | Result |
| --- | --- |
| `Keep running` button | Prevent closing; the desktop remains open. |
| `Stop CARE and quit` | Attempt a bounded Compose stop, then allow closing if it succeeds. |
| No clinic detected, dialog error, or prompt timeout | Allow closing without claiming that containers were stopped. |

The prompt timeout is 10 seconds; the explicit stop timeout is 90 seconds. The `closing` flag is set under the operation lock before an allowed exit. New protected jobs and reads reject it.

These semantics are separate from macOS window hiding: hiding a window is not necessarily process shutdown.

## Concurrency and job protocol

### Three execution helpers

| Helper | Lock | Completion contract |
| --- | --- | --- |
| `run(fn, markSetup, label)` | Exclusive `TryLock` held by the job goroutine | Method returns after acceptance; completion uses events. |
| `withJob(fn)` | Exclusive `TryLock` | Method returns the callback's result. |
| `withReadJob(fn)` | Shared `TryRLock` | Method returns the read result; other protected readers may run simultaneously. |

All three reject a closing application. Failed lock acquisition returns `something else is still running - wait for it to finish`. There is no queue or automatic retry.

The shared helper is used by `ReadEnv` and `ReadPlugins`. It prevents the two environment reads and plugin reads from rejecting one another, while still excluding setup, removal, and writes. It does not allow settings reads during an active exclusive job.

```mermaid
sequenceDiagram
    participant UI as Desktop
    participant A as App
    participant J as Job goroutine
    participant C as Clinic
    UI->>A: ClinicAction(action, password)
    A->>A: Validate action name and TryLock
    alt Busy or closing
        A-->>UI: Rejected promise
    else Accepted
        A->>J: Start job while holding lock
        A-->>UI: Resolved promise: accepted, not completed
        J->>J: Check lifecycle and any required authorization
        J->>C: Perform requested operation
        C-->>UI: care-log via App.logln
        C-->>J: Success or error
        J-->>UI: Error log and optional native dialog on failure
        J-->>UI: care-done with 0 or 1
        J->>J: Release lock on goroutine exit
    end
```

The final event is emitted by a deferred finalizer, before the outer deferred unlock. It is a completion notification, not a reservation for an immediately chained mutation. Callers must still handle a rejected subsequent request.

For setup, `markSetup=true` means the runner persists `SetupDone=true` only after the setup callback succeeds. It then emits `setup-done` and shows the installed-clinic dialog. If persisting the successful state fails, the job is still reported as failed.

A job can be waiting for a native confirmation or result dialog while holding the lock. An idle-looking terminal does not prove that the job has finished; both the work and its synchronous dialog handling must return before the lock is released.

A panic inside the job is converted to a logged stack trace and failure notification. This recovery is for the long-running job boundary; it is not a general transaction rollback.

### Lifecycle and authorization guards

| Guard | What it requires |
| --- | --- |
| `requireAdmin(password)` | The cached configuration has `SetupDone=true` and bcrypt accepts the password against `AdminPwHash`. |
| `requireSetup()` | Not removing, setup complete, and the installed `docker-compose.yml` exists as a regular file. |
| `requireStableClinic()` | `requireSetup()` plus no pending restore journal. |

Start and Stop can be requested during a pending restore; Start performs recovery. Restart, rebuilds, backup-now, settings writes, plugin writes, backup-directory changes, and new restores require a stable clinic.

Environment and plugin reads use `requireAdmin` plus `requireSetup`, allowing inspection after an interrupted restore. They still respect the shared/exclusive operation gate.

The desktop administrator password is a local authorization mechanism. The interface may check it to unlock controls, but protected Go methods check it again. There is no durable unlocked session or bearer token.

## Complete bound method reference

The following methods are declared in [`wails.d.ts`](../app/frontend/src/wails.d.ts). Types below use their JavaScript-facing names; every call returns a promise. An `error` return from a synchronous Go method rejects that promise.

Execution abbreviations: **query** means no `run`/`withJob` helper, **read** means shared read gate, **sync** means exclusive synchronous gate, and **job** means exclusive asynchronous job.

### State and prerequisite operations

| Method | Result | Execution and contract |
| --- | --- | --- |
| `GetState()` | `AppState` | Query. Returns embedded version, setup availability, name, Docker status, and pending restore status; journal errors propagate. |
| `DockerStatus()` | `DockerStatus` | Query. Checks actual Docker/Compose usability. |
| `GitStatus()` | `DockerStatus` | Query. Uses the same `{ok, message}` shape for Git. |
| `MDNSStatus()` | `NameStatus` | Query. Reports whether this process has an advertiser, not an end-to-end remote-device verdict. |
| `NetworkStatus()` | `NetworkStatus` | Query. Platform-specific networking inspection. |
| `FixNetwork()` | `void` | Sync. Runs the native networking repair. |
| `DockerPlan()` | `ToolPlan` | Query. Describes the available Docker install/open/manual action. |
| `GitPlan()` | `ToolPlan` | Query. Describes the available Git action. |
| `InstallDocker()` | `string` | Sync. Runs prerequisite provisioning and returns its result or error. |
| `InstallGit()` | `string` | Sync. Runs Git provisioning. |
| `OpenDocker()` | `void` | Sync. Attempts to launch the available Docker application. |
| `RestartPlan()` | `RestartPlan` | Query. Describes a detected prerequisite-related reboot requirement. |
| `RestartNow()` | `void` | Sync. Attempts to enable login startup, then requests an OS restart; autostart failure is logged. |
| `ClinicHealth()` | `Health` | Query. HTTP health probe, separate from name and certificate-trust checks. |
| `ClinicStatus()` | `string` | Query. Engine's formatted Compose service status. |

### Setup and lifecycle

| Method | Result | Execution and contract |
| --- | --- | --- |
| `ValidatePassword(pw)` | `string` | Query. Empty string means valid; otherwise a policy message. |
| `ValidateDomain(name)` | `string` | Query. Empty string means a valid clinic label. |
| `ValidateBackupDir(dir)` | `string` | Query-like validation with filesystem inspection and a temporary write probe. Empty input selects the default destination for validation. |
| `SetMDNSName(name)` | `void` | Sync. Pre-setup only; rejects installed or removing state, saves the normalized `.local` name, restarts advertising. |
| `VerifyAdminPassword(pw)` | `boolean` | Query. Compares with the saved desktop bcrypt hash. |
| `RunSetup(mdnsName, adminPassword, backupPassword, backupDir)` | `void` | Job. Validates passwords/name, requires no installed/removing clinic, prepares configuration and kit, sets up and starts the engine. |
| `CleanupFailedInstall()` | `void` | Sync. Only for an incomplete setup; safely removes failed-install resources while retaining required recovery material. |
| `ClinicAction(action, adminPassword)` | `void` | Job. Allow-listed action dispatch; details below. |
| `RunUninstall(removeImages, removeBackups, adminPassword)` | `void` | Job. Requires local admin authorization; persists removal state before destructive work. |
| `ScanResidue()` | `ResidueReport` | Query. Inspects owned files, Docker resources, saved password presence, and native traces. Inspection errors propagate. |
| `PurgeResidue()` | `void` | Sync. Refuses a normal installed clinic; requires a native destructive confirmation when residue exists. Preserves backups. |

The password policy in [`password.go`](../app/password.go) is 8 through 20 Unicode characters, with at least one uppercase letter, lowercase letter, and digit. Setup checks both administrator and backup passwords.

`ClinicAction` accepts only:

| Action | Engine call | Additional rule |
| --- | --- | --- |
| `start` | `Start()` | Setup required; pending restore allowed for recovery. |
| `stop` | `Stop()` | Setup required; pending restore allowed. |
| `restart` | `Restart()` | Stable clinic required. |
| `rebuild-backend` | `RebuildBackend()` | Stable clinic and administrator password required. |
| `rebuild-frontend` | `RebuildFrontend()` | Stable clinic and administrator password required. |
| `backup-now` | `BackupNow()` | Stable clinic required. |

The API does not require the desktop admin password for every operational control. In particular, ordinary start/stop/restart, backup-now, and backup-directory changes have their lifecycle checks but not `requireAdmin`.

### Environment, plugins, and backups

| Method | Result | Execution and contract |
| --- | --- | --- |
| `ReadEnv(name, adminPassword)` | `string` | Read. Admin plus setup required; `name` is only `backend` or `frontend`. Returns installed file contents. |
| `WriteEnv(name, content, adminPassword)` | `void` | Sync. Admin plus stable clinic required; parse dotenv syntax, then atomically replace the selected file. |
| `ReadPlugins(adminPassword)` | `CarePlugin[]` | Read. Admin plus setup required; parse `ADDITIONAL_PLUGS` from installed `backend.env`. |
| `SavePlugins(plugins, adminPassword)` | `void` | Sync. Admin plus stable clinic required; update the plugin variable without rebuilding by itself. |
| `ListBackups()` | `Backup[]` | Query. Returns an empty list if the installed compose file is absent; other file/read errors are not treated as an empty list. |
| `GetBackupDir()` | `string` | Query. Effective backup directory, including the engine's default if unconfigured. |
| `SetBackupDir(dir)` | `string` | Sync. Stable clinic required. Takes a parent folder, appends `care-db-backups`, preserves the key and conditionally restarts the sidecar. |
| `ChooseBackupFile()` | `string` | Native dialog. Starts in the backup directory; empty string means canceled or dialog failure. |
| `InspectBackupFile(path)` | `ImportedBackup` | Query. Checks filename/regular-file metadata and matching neighboring archive/key; does not yet validate dump contents. |
| `RestoreFromFile(path, passphrase, adminPassword)` | `void` | Job. Admin plus stable clinic required; inspect chosen file, obtain saved password if needed, restore from its directory. |
| `RestoreBackup(dbDump, filesArchive, passphrase, adminPassword)` | `void` | Job. Admin plus stable clinic required; restore selected names from the configured backup directory. |

File selection is not restore authorization. Full validation and data replacement occur in the later protected restore job. See [backups and restore](backups-and-restore.md).

### Desktop utilities

| Method | Result | Execution and contract |
| --- | --- | --- |
| `OpenURL(url)` | `void` | Opens the URL through the native browser integration. |
| `ChooseFolder(title)` | `string` | Native directory dialog; empty string means canceled or dialog failure. |
| `LogPath()` | `string` | Current diagnostic log path; may be empty if file logging is unavailable. |
| `OpenLogFolder()` | `void` | Opens/reveals the log with the OS file browser; errors if no log file is available. |
| `WasAutostartLaunched()` | `boolean` | Whether process arguments contain `--autostart`. |
| `AutostartEnabled()` | `boolean` | Reads the platform's login-startup state. |
| `SetAutostart(on)` | `void` | Sync. Changes the platform login-startup entry. |

## Events and result shapes

| Event | Payload | Meaning |
| --- | --- | --- |
| `care-log` | One string | A line already written by Go to the host log. Do not write it back to the host again. |
| `care-done` | Number `0` or `1` | An asynchronous App job succeeded or failed. Not a detailed subprocess exit code. |
| `setup-done` | `true` | Setup callback and persistence of `SetupDone` succeeded. |
| `uninstalled` | `true` | Normal uninstall completed its cleanup and local state removal. |

There is no job identifier or structured progress event. The single-job model keeps completion unambiguous, and the desktop derives setup progress from log messages using [`run-steps.ts`](../app/frontend/src/lib/run-steps.ts). Changes to important setup messages can therefore affect displayed progress.

Lines such as `$ care start` in the desktop log are action labels written by the state store. They do not imply that this repository ships a separate `care` command-line program.

The core serialized shapes are:

| Shape | Fields |
| --- | --- |
| `AppState` | `version`, `setup_done`, `mdns_name`, `docker`, `restore_pending`. `setup_done` is false while removal is in progress. |
| `DockerStatus`, `NameStatus` | `ok`, `message`. |
| `Health` | `active`, `code`, `detail`. |
| `NetworkStatus` | `applicable`, `ok`, `message`, `how`, `fixable`. |
| `ToolPlan` | `action`, `label`, `detail`, `url`. |
| `RestartPlan` | `needed`, `title`, `detail`, `label`. |
| `ResidueReport` | `clean`, `traces`; each trace has `id`, `label`, `detail`. |
| `Backup` | `db_dump`, `files_archive`, `label`, `manual`, `encrypted`, `size_bytes`. |
| `ImportedBackup` | `path`, `dir`, `db_dump`, `files_archive`, `label`, `encrypted`, `has_key`. |
| `CarePlugin` | `name`, `package_name`, optional `version` and `configs`. |

Go structs and their JSON tags are authoritative. [`types.ts`](../app/frontend/src/types.ts) mirrors them for the desktop.

## JavaScript bridge behavior

[`bridge.ts`](../app/frontend/src/lib/bridge.ts) resolves `window.go.main.App` per call rather than at module import. It waits up to 10 seconds, polling every 25 milliseconds, for runtime injection; the resolved object is cached. Missing methods produce an error naming the method instead of an opaque JavaScript invocation error.

`onCareEvent()` also waits for runtime injection and returns a cancellation/unsubscription function. `logToHost()` is only for messages originating in the desktop interface; replaying `care-log` through it would duplicate output.

The state store subscribes to completion events, clears its local busy state, refreshes health/backups/pending-restore state, and returns to setup if the backend reports the installation unavailable. It polls panel health every five seconds, skipping refresh while an action is busy.

These are integration details, not an alternative source of backend truth. A stale React state value cannot override a Go lifecycle guard.

## Changing this boundary

When changing an exported `App` method, update its declaration in `wails.d.ts`, its call sites, and any changed data shape in `types.ts`. The existing binding check verifies method names and argument counts, not full semantic or return-type compatibility.

Use `run()` only when completion should be event-driven. Do not make a synchronous write start an untracked goroutine, or make a read take an exclusive job lock just because a write in the same file uses it. Keep authorization and lifecycle checks inside the protected operation to avoid check-then-act races.
