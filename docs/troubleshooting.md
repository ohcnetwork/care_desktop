# Troubleshooting

Common problems and fixes. Most issues are one of: Docker not running, `care.local`
not resolving, or the MinIO endpoint not reachable from other devices.

---

## `care.local` doesn't open on a phone/laptop

**Cause:** the name isn't being advertised, or the device doesn't speak mDNS.

- **Confirm the server's name:**
  - macOS: `scutil --get LocalHostName` → must be `care`. Fix: `sudo scutil --set LocalHostName care`.
  - Linux: `hostname` → must be `care`, and `systemctl status avahi-daemon` active.
  - Windows: rename the PC to `care` (Settings → System → About → Rename), set the network to **Private**, and allow **UDP 5353** in Windows Firewall. Recent Windows 11 then advertises `care.local` natively; older builds need **Apple Bonjour**.
- **Test from the server itself:** open `http://care.local/` on the server's own browser. If that works but phones don't, it's the device's mDNS.
- **Fallback — use the IP:** find the server's IP (`ipconfig` / `ip addr` / `ifconfig`) and open `http://<ip>/`. If you'll use the IP permanently, also set `BUCKET_EXTERNAL_ENDPOINT=http://<ip>:9100` in `backend.env` and rebuild the frontend with `REACT_CARE_API_URL=http://<ip>` (see [configuration.md](configuration.md)).
- Give the server a **DHCP reservation** in the router so its IP never changes.

---

## The installer's step 3 (`care.local`) won't go green

- You set the name but didn't click **Check** again — click it.
- On the Mac, the GUI can't run `sudo` — set the name in **Terminal** once (`sudo scutil --set LocalHostName care`), then **Check**.
- On Windows, install **Bonjour** and rename the PC to `care`, then **Check** (or use a static IP via the CLI).

---

## Docker step is red / "Docker is installed but not running"

- Open **Docker Desktop** and wait until it says *running*, then click **Check**.
- Linux: `sudo systemctl start docker`; make sure your user is in the `docker` group (`sudo usermod -aG docker $USER`, then re-login).

---

## File uploads or image previews fail (but the rest works)

**Cause:** `BUCKET_EXTERNAL_ENDPOINT` points somewhere devices can't reach (often
`localhost`).

- It must be a host **every device** can resolve: `http://care.local:9100` (default)
  or `http://<server-ip>:9100`.
- Port **9100** must be open on the server. Check `care status` shows `minio running`.
- After changing it in `backend.env`, run `care start`.

---

## "Install & Start" ran but the app isn't reachable

- Run `care status` (or check the panel). All services should be `running`.
- If `backend` keeps restarting, the database may still be initializing — wait a
  minute and `care restart`. Migrations retry automatically for ~100s on first start.
- Check the log pane (or `docker compose -p care-desktop logs backend`) for the error.

---

## First setup fails partway (network / build error)

- Setup needs **internet** to clone the repos and pull base images. Confirm connectivity.
- Re-run **Install & Start** (or `care setup`) — it's safe to repeat: existing clones
  and images are reused, and the secret/admin steps are idempotent.
- Low disk space breaks image builds — you need ~10 GB free.

---

## I changed `frontend.env` but nothing changed

The frontend bakes its settings at **build** time. Run `care rebuild-frontend` (or
**Save & rebuild** in the app) — a plain `start` won't pick up frontend changes.

---

## I forgot the admin password

Create or reset a superuser directly:
```bash
docker compose -p care-desktop exec backend python manage.py changepassword admin
# or create another superuser:
docker compose -p care-desktop exec backend python manage.py createsuperuser
```

---

## Did I lose data after stop/rebuild?

No — `care` never deletes volumes. Data survives `stop`, `start`, `restart`,
`rebuild-backend`, and `rebuild-frontend`. The only ways to lose data are removing the
Docker volumes manually (`docker compose -p care-desktop down -v`) or a disk failure
(hence: [keep backups on a separate drive](backups.md)).

---

## Reset everything and start clean (testing)

```bash
docker compose -p care-desktop down -v --remove-orphans   # removes containers + volumes
docker rmi care:clinic care_fe:clinic                    # force a rebuild next setup
# remove saved app state (macOS path shown):
rm -rf ~/Library/Application\ Support/care-desktop
```
Then launch the app (or `care setup`) for a fresh install. **This deletes all data —
only do it intentionally.**
