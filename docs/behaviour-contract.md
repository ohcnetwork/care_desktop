# Behaviour contract

Rules the backend must obey, extracted from the working implementation in
`care-clinic/app`. Each was learned from a real failure. The new code may be
structured however we like, but every rule here must survive — or be removed
**deliberately**, with a note saying why.

This is a checklist, not documentation. When a rebuilt package is reviewed, the
question is: which of these does it uphold, and where?

Severity:

- **DATA** — violating this corrupts or destroys clinic data.
- **SECURITY** — violating this opens an attack path.
- **BREAKS** — violating this produces a non-working install.
- **COST** — violating this wastes significant time or disk.

---

## 1. Setup (one-time bootstrap)

### S1 — `applyDomain` runs before `buildFrontend` · BREAKS

Vite bakes `REACT_*` variables into the frontend bundle at **image build time**.
The hostname must be written to `frontend.env` before the image is built.

*Violation:* a frontend image that points at the wrong API URL. Invisible until
a browser loads it, and unfixable without a rebuild.

### S2 — the source is cloned into a temporary directory, per build · (correctness)

A clone kept beside the install is only ever reused, never refreshed, so the built
image silently stays pinned to whichever ref was current the first time and ignores
every later change to `CARE_BE_REF` / `CARE_FE_REF`. `clone` makes a fresh shallow
checkout and removes it afterwards. It also keeps the install dir small, which is
the build context for the backup and Caddy images.

*Violation:* a release that bumps a source ref keeps building the old code.

### S3 — `genSecret` replaces `DJANGO_SECRET_KEY=CHANGE_ME` exactly once · SECURITY

Guarded by a substring check, so re-running setup never rotates the key on a live
install. Uses `crypto/rand`, not a shell or Python call.

*Violation:* rotating the key invalidates every active session and any
signed value in the database.

### S4 — hosts entry and CA trust are deferred to `Start` · BREAKS

Not done during setup, for two independent reasons: the root CA does not exist
until Caddy has run at least once, and deferring keeps the whole install to a
**single** admin/sudo approval.

*Violation:* trusting a certificate that isn't there yet, plus a second
credential prompt mid-install.

---

## 2. Start

### R1 — single migrator · DATA

The order is fixed:

```
up -d db redis backend   →   migrate   →   up -d
```

The API container's `start.sh` does **not** migrate, so we do. But
**celery-beat's entrypoint does** migrate on boot. Bringing everything up at once
runs two migrators concurrently against one database.

*Violation:* `column ... already exists`, and a partially-applied migration set
on a production database. This is the single most dangerous rule in the file.

### R2 — `migrate` failing is fatal · DATA

It retries 20× at 5s, then returns an error. It must not be best-effort.

*Violation:* the installer reports success over a half-migrated stack.

### R3 — `ensureKeysDir` runs before `up` · BREAKS

`./keys` is a bind-mount source. Docker creates missing bind sources as
root-owned directories. An empty dir is the valid "encryption off" state.

*Violation:* a root-owned path the containers cannot read.

### R4 — success is reported only after `WaitHealthy` · BREAKS

`up -d` means *containers created*, not *application answering*. The installer
uses this as the gate for marking the install complete.

*Violation:* "CARE is up" shown for a stack that is still booting or crash-looping.

### R5 — `writeCertInstallers` and `ensureLocalAccess` run after `WaitHealthy` · BREAKS

The root CA lives in the `caddy-data` volume and does not exist until Caddy has
started.

*Violation:* an empty or missing CA file gets installed into the system trust store.

### R6 — `createAdmin` is idempotent and captures its output · (correctness)

On an existing install `createsuperuser` exits 1 with "username already taken" —
the normal case. Output is captured rather than streamed so a raw
`CommandError ... exit status 1` never reaches the user's log.

---

## 3. Rebuild

### R7 — `RebuildBackend` repeats the single-migrator order · DATA

```
build   →   up -d backend   →   migrate   →   up -d celery-worker celery-beat
```

Same reason as R1: the freshly built code may carry new migrations, and
celery-beat would race them.

### R8 — `RebuildFrontend` exists because Vite bakes at build time · BREAKS

Changing a `REACT_*` value requires an image rebuild, not a restart. Any code
path that edits frontend env must trigger a rebuild, not `up -d`.

---

## 4. Restore

### B1 — filenames are validated by regex before reaching a shell · SECURITY

Dump and archive names are interpolated into `sh -c` scripts inside containers.
They are first reduced with `filepath.Base` (tolerating a pasted path) and then
matched against a strict allowlist:

```
^(?:care-(?:manual-)?\d{8}-\d{6}\.dump(?:\.enc)?|files-\d{8}-\d{6}\.tar\.gz(?:\.enc)?)$
```

No separators, no metacharacters. The prefix (`care-` / `files-`) is checked
separately so a dump can't be passed as an archive.

*Violation:* command injection with the privileges of the backup container.

### B2 — passphrase and key are checked before anything destructive · DATA

Both are validated **up front**, before services are stopped and before the
database is dropped.

*Violation:* the database is dropped, then the restore fails on a wrong password,
leaving no database at all.

### B3 — an encrypted dump is decrypted to a tmpfile before the drop · DATA

Decryption happens first inside the container script, with a `trap` to remove the
tmpfile. A wrong key therefore fails while the live database is still intact.

### B4 — app services stop before the swap · DATA

`stop backend celery-worker celery-beat` releases DB connections and halts
writes. `restoreDB` additionally calls `pg_terminate_backend` on stragglers
before `dropdb`.

*Violation:* `dropdb` fails on open connections, or writes land mid-restore.

### B5 — file restore uses a throwaway container · DATA

The long-running `backup` container mounts `minio-data` **read-only on purpose**.
Restoring files uses a separate `docker run --rm` with a read-write mount, and
stops `minio` first so nothing is mid-write. The backup image is reused because
its busybox has `tar` + `gzip`, so decrypt and extract work fully offline.

*Violation:* making the standing backup container's mount read-write gives a
long-lived process the ability to erase uploads.

### B6 — restore re-applies the single-migrator order · DATA

The dump is older than the running code, so it always has pending migrations —
the exact condition that triggers the celery-beat race. See R1.

---

## 5. Uninstall

### U1 — the root CA is captured before `compose down -v` · (cleanup correctness)

It lives in the `caddy-data` volume, which `down -v` destroys. Read it into
memory first so it can be untrusted at the end.

*Violation:* a trusted root certificate permanently stranded in the system store.

### U2 — `forceRemoveProject` is nested inside the "compose file exists" check · DATA

It matches on the **compose project label**, not on this install dir. Called
unguarded, it deletes the containers and data volumes of *any* CARE install on
the machine.

*Violation:* destroying a different clinic install's database. Second most
dangerous rule here — and the guard looks removable, which is what makes it
dangerous.

### U3 — never delete a source checkout · (developer safety)

`looksLikeSourceRepo` checks for `.git` / `app` / `docs` markers **and walks up
the tree** for a `.git` ancestor, because a developer's install dir can be the repo
root itself.

*Violation:* `RemoveAll` on a developer's working tree.

### U4 — the image list is derived, never hardcoded · (regression)

`uninstallImages()` builds its list from the same accessors that tagged the
images. Hardcoding it is what made `--images` silently match nothing once
`.env` started pinning real versions.

### U5 — build-cache pruning happens only under `RemoveImages` · COST

The cache is machine-wide and carries no project label, so there is no way to
remove only ours. Tens of GB — larger than the images. Gated behind the option
that already means "take the downloads with it".

### U6 — every step is best-effort; the run ends with an honest report · (trust)

One failing step must not abort the rest, so a half-finished install can still be
cleaned up. The close-out names what could not be reverted (hosts line, trusted
root) and what was kept on purpose. "Uninstall complete" alone is a claim the
operator cannot verify.

---

## 5a. Residue scan and purge (first run only)

A prerequisite check in the setup wizard, alongside Docker and Git. It exists
because Docker volumes outlive the app that made them: an old `care-desktop`
data volume is re-attached by name on the next `compose up`, so a "fresh" setup
silently comes up holding the previous clinic's patients. Nothing in the setup
flow would surface that.

### P1 — detection never elevates and never mutates · (trust)

`residue.Scan` reads only what is readable as the current user. A CA root
visible only to root is reported absent rather than raising a password prompt
the operator did not ask for. A scan is a question, not an action.

### P2 — backups are never scanned and never purged · DATA

They are the recovery data; removing them to "tidy up" is indefensible, and the
operator has no way to get them back. This also keeps the post-purge rescan
honest — a trace that is deliberately never removed could never report clean.

### P3 — `Purge` calls `forceRemoveProject` **unguarded**, diverging from U2 · DATA

U2 guards that call because `Uninstall` runs against an install this app owns,
where the install dir is the authority on what to remove. Purge runs from the
first-run wizard, before this app owns anything: there is no compose file to
guard on, and removing whatever still carries the project label is the entire
point. The two callers have opposite requirements, which is why this is a
separate function rather than a flag on `Uninstall`.

*What keeps it safe:* P4 below, plus U3 — `Purge` honours `looksLikeSourceRepo`.

### P4 — purge refuses once `setup_done` is true · DATA

Checked in Go (`App.PurgeResidue`), not only in the wizard. Once the install is
this app's own, "leftovers" *are* the live clinic, and P3's unguarded removal
would destroy it. The supported way to remove an owned install is Uninstall,
which names what it does and asks about backups.

### P5 — the row re-checks after Docker comes up · (correctness)

Container, volume, and image traces are invisible while the daemon is down, so a
scan run first would under-report and clear the row. `recheckAll` re-runs the
scan whenever any check action finishes, which covers the Docker install case.

---

## 6. Name resolution

### H1 — `care.local` is advertised, never assigned · (reversibility)

The app answers mDNS itself with a pure-Go responder for as long as it is open. It
does not rename the machine, install Avahi, or edit any OS-level hostname, so there
is nothing to undo at uninstall and no sudo prompt during setup.

*Violation:* an uninstall leaves the clinic's name on the operator's computer.

---

## 7. Cross-cutting

### X1 — `composeProject` is `care-desktop` · DATA

Volumes are named `<project>_<volume>` (e.g. `care-desktop_minio-data`), and the
label-based teardown in U2 keys off it. Changing this string orphans every
existing volume on every existing install.

### X2 — line splitting must tolerate CRLF · BREAKS

`.env` parsing currently splits on `"\n"` only. A file edited by a Windows tool
leaves a trailing `\r` on every value. Trim explicitly: `strings.Trim(s, "\r\n")`.
*(Known defect in the current code — fix during the rebuild.)*

### X3 — `path/filepath` for host paths, `path` for container paths · BREAKS

Container-internal paths, URLs, and embedded FS paths are always `/`-separated
and must use `path`. Host paths must use `filepath`.

### X4 — the engine carries no Wails dependency · (architecture)

The orchestration layer is UI-agnostic and never imports Wails. Logging and
confirmation are **injected**. This is the best decision in the existing codebase
and must survive: it is what makes the backend testable and scriptable.

---

## 8. Known defects — fix during the rebuild, don't reproduce

All of D1-D8 were fixed in the rebuild. They are kept here because each one is a
mistake the old code made for a plausible-looking reason, and a future change
could reintroduce it.

| | Issue |
|---|---|
| D1 | `App.run(e *care.Engine, ...)` never uses `e`. Dead parameter at every call site. |
| D2 | `type errString string` reinvents `errors.New`. 9 call sites. |
| D3 | `WaitHealthy`'s initial detail and `EnsurePortFree`'s doc comment still say `:80`; the probe moved to `:443`. |
| D4 | PATH is set twice — `FixPath()` on the process, then `augmentedPath()` again per command. |
| D5 | `startup()` calls `startAdvertise()` before `advStop` exists. Theoretical nil-close race. |
| D6 | The action list is duplicated between the `allowed` map and the `actionFunc` switch, with nothing keeping them in sync. |
| D7 | Restore always passes `passphrase=""`; `BackupEncryptionEnabled` and `HasStoredBackupPassword` exist for the UI to prompt but are never called. Encrypted restore is impossible without a keychain entry. **Resolved by wiring the prompt**, not by deleting it - see §9. |
| D8 | No CI. Nothing runs `go test` or `go vet`. |

---

## 9. Dead code — do not port

Confirmed unreferenced by the frontend (verified against every call site):

- `ListApps`, `SetAppEnabled` — and with them effectively all of `apps.go`
  (~280 of 344 lines), plus `apps.json` and its staging entry. Keep only
  `plugRows`, `rowsPrefix`, `slugRe`, `host()`, which `fe_plugins.go` shares.
- `ConfirmUninstall`, `ConfirmRestore` — replaced by in-app React confirmations.
- `BackupEncryptionEnabled`, `HasStoredBackupPassword` — see D7. **Decided:**
  encrypted restore was wired up rather than dropped (the restore row shows a
  password field when the backup is `encrypted`, blank falling back to the
  keychain), and both bindings were then deleted as redundant - `encrypted` is
  already per-backup, and `RestoreBackup` does the keychain fallback itself.
