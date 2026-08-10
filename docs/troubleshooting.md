# Troubleshooting

Common problems and fixes. Most issues are one of: Docker not running, `care.local`
not resolving, or the MinIO endpoint not reachable from other devices.

---

## `care.local` doesn't open on a phone/laptop

**Cause:** the name isn't being advertised, or the device doesn't speak mDNS.

- **Confirm the server's name:**
  - macOS: `scutil --get LocalHostName` → must be `care`. Fix: `sudo scutil --set LocalHostName care`.
  - Linux: `hostname` → must be `care`, and `systemctl status avahi-daemon` active.
  - Windows: **the server's own browser resolves `care.local` automatically** (CARE adds a `127.0.0.1 care.local` hosts entry at setup — no rename needed). For *other devices* to find it, set the network to **Private** and allow **UDP 5353** in Windows Firewall; recent Windows 11 then resolves `care.local` natively, older builds need **Apple Bonjour** or a static IP.
- **Test from the server itself:** open `https://care.local/` on the server's own browser (works out of the box on every OS — macOS/Linux via mDNS, Windows via the hosts entry). If that works but phones don't, it's the device's mDNS / your network settings.
- **Fallback — use the IP:** find the server's IP (`ipconfig` / `ip addr` / `ifconfig`) and open `https://<ip>/`. If you'll use the IP permanently, also set `BUCKET_EXTERNAL_ENDPOINT=https://<ip>` in `backend.env` and rebuild the frontend with `REACT_CARE_API_URL=https://<ip>` (see [configuration.md](configuration.md)). (Note: the cert is issued for `care.local`, so an IP URL shows a name-mismatch warning — prefer the name.)
- Give the server a **DHCP reservation** in the router so its IP never changes.

### …and it was working, then suddenly `DNS_PROBE_FINISHED_NXDOMAIN`

The in-app mDNS responder stopped answering (sleep/wake or a WiFi flap). The app has a
self-heal watchdog that re-advertises within ~60s of the name failing to resolve; if
it doesn't recover, **restart CARE Desktop** (or `care restart`) to re-advertise
immediately.

---

## The installer's step 3 (`care.local`) won't go green

- You set the name but didn't click **Check** again — click it.
- On the Mac, the GUI can't run `sudo` — set the name in **Terminal** once (`sudo scutil --set LocalHostName care`), then **Check**.
- On Windows, the server itself resolves `care.local` automatically (hosts entry). If the Check tests reachability from *other* devices and it's red, set the network to **Private** + allow **UDP 5353**, or install **Bonjour** / use a static IP, then **Check**.

---

## Docker step is red / "Docker is installed but not running"

- Start your Docker engine and wait until it's running, then click **Check**. (Docker Desktop: open it, wait for "running". Colima: `colima start`.)
- Linux: `sudo systemctl start docker`; make sure your user is in the `docker` group (`sudo usermod -aG docker $USER`, then re-login).
- "Compose plugin is missing": install `docker compose` v2 — bundled with Docker Desktop, but a separate package (e.g. `docker-compose-plugin`) on some Docker Engine / Colima setups.
- **Windows: "Docker not found" even though you installed it in WSL.** The Windows CARE Desktop app is a native Windows process and looks for `docker.exe` on the Windows PATH. Docker installed *inside* a WSL Ubuntu distro (`apt install docker.io`) is a Linux binary the Windows app can't see. Fix: install **Docker Desktop for Windows** and enable **WSL 2 integration** for your distro (Settings → Resources → WSL Integration) — that puts a real `docker.exe` on the Windows PATH. Then quit and reopen CARE Desktop so it re-reads PATH.

---

## Security warning / red padlock instead of a green one

**Cause:** the device hasn't trusted the clinic's local CA (or trusted an old one).

- **Trust the cert:** open **`https://care.local/setup`**, pick the device, and follow
  the steps (download `root.crt` → add it to the system trust store). The server
  machine does this automatically on start; only *other* devices need it.
- **iOS is two steps:** installing the profile is **not** enough — after installing,
  go to **Settings → General → About → Certificate Trust Settings** and toggle
  **"CARE Desktop Local CA"** on. The profile install must be done in **Safari**
  (Chrome/other browsers on iOS can't install profiles).
- **Still red on Chrome after trusting?** Chrome caches the cert-error state. **Fully
  quit** Chrome (not just the tab) and reopen; if it persists, clear the domain at
  `chrome://net-internals/#hsts` (Delete `care.local`).
- **Re-installed / re-set-up the stack?** A fresh setup mints a **new** CA, so devices
  that trusted the old one must re-trust from `/setup` (and remove the stale cert).
- **`/setup` opens Google search on Safari?** Type the full `https://care.local/setup`
  (a bare `care.local/setup` is treated as a search term).

---

## The download button on `/setup` gives a redirect page, not the cert

The `/root.crt` file is gated so people go through the instructions. Use the page's
**Download** button (it carries the `?ok=1` marker) — don't hit `/root.crt` directly,
and don't `curl` it without `?ok=1` (you'll save the redirect HTML and get
`Error reading file` when you try to trust it). With curl:
`curl -o root.crt 'https://care.local/root.crt?ok=1'`.

---

## File uploads or image previews fail (but the rest works)

**Cause:** `BUCKET_EXTERNAL_ENDPOINT` points somewhere devices can't reach (often
`localhost`).

- It must be a host **every device** can resolve: `https://care.local` (default)
  or `https://<server-ip>`. Files are served through Caddy on the same origin as the
  app — so no extra port is involved.
- Check `care status` shows `minio running` (Caddy proxies to it).
- After changing it in `backend.env`, run `care start`.

> If previews fail with a *camera/scanner* error rather than a network error, the page
> isn't a secure context — confirm you're on `https://` and the cert is trusted (above).

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
- On Windows, **Try again** first tears down leftover containers and wipes the
  half-staged kit, so the retry starts clean.
- Low disk space breaks image builds — you need ~10 GB free.

---

## Windows: backend crash-loops with `ModuleNotFoundError: No module named 'clinic_settings'` (or "is a directory")

Docker Desktop's WSL2 file share can't read files created under `%AppData%` on some
Windows setups, so the bind-mounted `clinic_settings.py` arrives as an **empty
directory** and the backend can't start. The app avoids this by staging the kit under
your **home dir** (`%USERPROFILE%\care-desktop\kit`), which Docker reads live — so a
current install shouldn't hit this. If you see it on an older install, uninstall and
reinstall so the kit is re-staged to the home dir.

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
