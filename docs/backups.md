# Backups & restore

CARE Desktop backs up **automatically, every day**, with no setup. This page covers
where backups go, how to restore them, and how to keep them safe.

---

## What's backed up

A dedicated `backup` container runs continuously and, every 24 hours (and once at
startup), writes **two files** with a timestamp:

| File | Contents |
|---|---|
| `care-<timestamp>.dump.enc` | the full PostgreSQL database (patient records, users, everything) — `pg_dump -Fc`, encrypted |
| `files-<timestamp>.tar.gz.enc` | the uploaded files from MinIO (X-rays, documents, logos), encrypted |

Both are needed for a full restore — the database alone won't bring back an X-ray.

**The two are written as a set, and a set is all-or-nothing.** Each is verified before
it counts (`pg_restore --list` for the dump, `tar -tzf` for the archive), and old
backups are deleted **only after a complete, verified set has landed**. If either half
fails, nothing is pruned — a failed backup can cost disk, but it can never cost a
known-good backup.

> On-demand: click **Backup now** in the app. That writes an immediate
> `care-manual-<timestamp>.dump.enc` — encrypted and verified like the daily one,
> but database-only, so it is not a complete set.

---

## Where they go

- **Default:** `~/Desktop/care-db-backups/`
- **Chosen:** whatever folder you picked in the installer's "Backup location" step

### Changing it later

You are not stuck with the folder you picked at installation. In the app, open
**Backups** — the top row shows where backups are being written, with a **Change**
button that opens a folder picker. Point it at a USB or external drive.

What happens when you change it:

- New backups go to the new folder from the next cycle. The backup service is
  restarted so it picks the change up immediately — a container's folder is fixed
  when it starts, so this restart is what makes the change take effect.
- **The recovery key is copied across**, so the new folder is self-contained and can
  be restored on a different computer.
- **Existing backups are left where they are.** They can be many gigabytes, and the
  old drive may be the one you are moving away from, so nothing is copied or deleted
  behind your back. Move them yourself if you want everything in one place. The app
  tells you which folder they were left in.


**Retention:** controlled by `DB_BACKUP_RETENTION_PERIOD` in `backend.env` (default
**14** days). Older `care-*.dump` and `files-*.tar.gz` are pruned automatically.

- **`0` means keep everything** — nothing is ever deleted. Watch the disk if you set this.
- **Whole numbers only** — `0`, `1`, `2`… The value goes straight to `find -mtime`,
  which does something unhelpful rather than complaining if given a negative or a
  decimal, so don't.
- **Nothing is pruned unless the whole backup succeeded.** Database *and* files, both
  verified. A failed backup costs disk; it never costs a known-good backup.
- Pruning is by age, not by count: there is no "always keep the last N". If the
  sidecar is stopped for longer than the window, the next run prunes everything
  past it in one pass.
- **Manual backups are pruned too.** `care-manual-*.dump` matches the same rule, so a
  "Backup now" taken before something risky is not kept indefinitely.
- The recovery key `backup-key.pem.enc` is never pruned.

> ⚠️ **Put backups on a separate drive.** Point the backup folder at a **USB or
> external drive** (or copy it there regularly). If the server's disk dies, backups
> on that same disk die with it. The backup *and* the data being on one disk is not a
> backup — it's a single point of failure.

---

## Encryption

**Every backup is encrypted.** There is no plaintext option and no way to switch it
off — a clinic's backups end up on a Desktop, get copied to USB sticks and synced into
cloud folders, and unreadable-without-the-password is the only acceptable state for
them to be in out there.

At installation you set a **backup password**, which is therefore required: setup will
not complete without one. From it, CARE generates an encryption keypair. The daily
`backup` container holds only the *public* half, so it can seal every dump but can
never open one — not even the ones it just wrote. Backups are written as `*.enc`.

This applies to **Backup now** as well, not just the daily run.

- **The password can't be recovered.** There is no reset and no back door — that's
  what makes the encryption real. If it's lost, every encrypted backup is lost with
  it. Write it down and keep it somewhere safe, off the machine.
- **You enter it once, at installation.** It's saved to this computer's OS keychain,
  so restores on this machine never ask for it again — there's no separate password
  prompt. (This does keep the password on the machine; the encryption's main job is
  protecting the *backup files* once they're copied off to a USB stick or synced to a
  cloud folder.)
- **The backup folder is self-contained.** A copy of the (password-protected) private
  key, `backup-key.pem.enc`, is dropped alongside the backups, so the folder can be
  restored on a *different* computer. There you run the installer, enter the same
  backup password in its backup section, and restore — the key travels with the folder.

> Backups made by an **older version**, before encryption was mandatory, may be
> plaintext. They keep restoring normally — the app handles a folder that mixes both.
> Only newly written backups are affected by the rule above.

---

## Restore a backup

Restore is **built in** — you don't run the SQL by hand. It stops the app services,
drops + re-creates the database from the dump, restores the uploaded files (when the
backup has them), then brings CARE back up and migrates.

Nothing destructive happens until the replacement is known to be good: the dump is
decrypted and checked with `pg_restore --list` *before* the database is dropped, and
the files archive is decrypted and checked with `tar -tzf` *before* the MinIO volume is
cleared. A wrong password or a damaged file therefore fails with your data still
intact.

> ⚠️ **Restoring replaces the current data and can't be undone.** Take a fresh
> **Backup now** first if you're unsure.

### In the app

Open the control panel → **Restore from a backup**. Pick a point from the dropdown
(each is labelled with its date, whether it's a *daily* or *manual* backup, whether
it includes files, and whether it's *encrypted*), click **Restore**, and confirm.
Encrypted backups need no password prompt — the one you set at installation is read
from this computer's keychain automatically. The app restarts itself when it's done.

> Match the **database** dump and the **files** archive from the **same timestamp**
> for a consistent restore (the automatic pairing does this for you).

### From a file on a USB drive (a backup from another computer)

The dropdown only lists backups in *this* clinic's backup folder. For one carried
over from a different machine, use **Restore from a file** at the bottom of the
Backups tab:

1. Click **Choose file** and pick the `care-…dump.enc` from the other clinic's backup
   folder.
2. The app checks the file before offering anything, and tells you what it found —
   whether the matching `files-…tar.gz.enc` is beside it, and whether the recovery
   key came along.
3. Enter that clinic's backup password, then confirm.

**Pick the file from inside the original backup folder, and keep the folder intact.**
A backup made on another computer was encrypted with *that* computer's key, and this
one's key cannot open it. The app reads the key from the folder the file sits in, so
copying a lone `.dump.enc` off a USB stick and leaving `backup-key.pem.enc` behind
gives you a file nothing can decrypt. If the key is missing, the app warns you before
you start.

<details>
<summary>Manual restore (fallback, if you ever need it)</summary>

> ⚠️ **These commands are for plaintext backups only** — ones written by an older
> version, before encryption became mandatory. A current `.enc` backup has to be
> decrypted first, which needs the private key and your backup password:
>
> ```bash
> docker run --rm -e BACKUP_PASS='your-backup-password' \
>   -v ~/Desktop/care-db-backups:/backups:ro \
>   care-backup:clinic sh -c 'openssl cms -decrypt -binary -inform DER \
>     -in /backups/care-YYYYMMDD-HHMMSS.dump.enc -out /backups/plain.dump \
>     -inkey /backups/backup-key.pem.enc -passin env:BACKUP_PASS'
> ```
>
> Use the app's restore instead unless you have a specific reason not to — it also
> verifies the dump before dropping anything, which these steps do not.

The built-in restore does exactly this. Stop the app first so nothing is writing:

```bash
docker compose -p care-desktop stop && docker compose -p care-desktop up -d db

docker compose -p care-desktop exec -T db psql -U postgres -c "DROP DATABASE IF EXISTS care;"
docker compose -p care-desktop exec -T db psql -U postgres -c "CREATE DATABASE care;"
cat ~/Desktop/care-db-backups/care-YYYYMMDD-HHMMSS.dump | \
  docker compose -p care-desktop exec -T db pg_restore -U postgres -d care

# files: extract the archive into the MinIO volume (care-desktop_minio-data)
docker run --rm \
  -v care-desktop_minio-data:/data \
  -v ~/Desktop/care-db-backups:/backup \
  alpine sh -c 'cd /data && tar -xzf /backup/files-YYYYMMDD-HHMMSS.tar.gz'

docker compose -p care-desktop up -d
```

</details>

---

## Moving the whole clinic to a new computer

1. On the old server: take a fresh backup (**Backup now**), copy the backup folder to a USB.
   If your backups are encrypted, that folder already contains `backup-key.pem.enc` —
   keep it there, and make sure you know the backup password.
2. On the new server: install CARE Desktop — it builds a fresh, empty stack.
3. Restore, either way round:
   - **Restore from a file** (Backups tab) — pick the dump straight off the USB. Nothing
     is copied and the USB folder is read-only throughout. Best when the USB is going
     back to the old machine.
   - Or point this clinic's **backup folder** at the USB (Backups → **Change**), which
     makes those backups appear in the normal dropdown, then restore from there. Best
     when the new machine is taking over the backup drive too.

   Either way you'll enter the old clinic's backup password. The key travels in the
   folder, so no other file from the old machine is needed — which is why the folder
   should be copied whole rather than file by file.

Because patient data lives in the Docker **volumes** (captured by the backups), this
moves everything — records and files — to the new box.
