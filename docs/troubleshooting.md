# Troubleshooting

Common problems and fixes. Most issues are one of: Docker not running, the clinic address
not resolving, or the MinIO endpoint not reachable from other devices.

---

## The clinic address doesn't open on a phone/laptop

**Cause:** DNS, the certificate, or the network path — check them in that order.

- **Does the name resolve?** On the phone (or any device on the clinic WiFi):
  `nslookup clinic.yourdomain.com`. It should return the server's address on the WiFi.
  - Nothing back → the `A` record is missing, or the domain isn't on Cloudflare's
    nameservers yet (`dig +short NS yourdomain.com`).
  - **A private IP is returned on the server but not on phones** → the router is doing
    **DNS rebinding protection**, which discards answers containing private addresses.
    Common on OpenWRT, Fritz!Box, pfSense and Pi-hole. Disable it, or add an exception
    for your domain.
- **Does it resolve to the *right* address?** If the server's IP changed (router
  reboot, new network), update the `A` record in Cloudflare and run `care restart`.
  Then add a **DHCP reservation** so it can't drift again.
- **Test from the server itself:** `curl -sI https://clinic.yourdomain.com/ping/`.
  `HTTP/2 200` means the server and certificate are fine and the problem is on the
  device or the network in between.
- **Firewall:** inbound TCP 443 must be allowed on the server.

---

## Browser shows a certificate warning

- **"Not private" / untrusted issuer** → you're on the Let's Encrypt **staging** CA.
  Staging certificates are deliberately untrusted. Remove `CARE_ACME_CA` from
  `tls.env` (or set it to the production URL) and run `care start`.
- **Name mismatch** → the address you typed isn't the one in `CARE_PUBLIC_HOST`.
- **Expired** → renewal has been failing. Check `docker compose logs caddy | grep -i acme`;
  usually the API token was revoked, or the domain left Cloudflare.

---

## Setup fails with a certificate error

```sh
docker compose logs caddy | grep -i -e acme -e error
```

- *"API token … appears invalid"* → the token was pasted with quotes or braces around
  it. Paste just the token.
- *403 / "invalid zone"* → the token lacks `Zone:DNS:Edit`, or is scoped to a different
  zone than the domain you entered.
- *timed out waiting for record* → the domain isn't on Cloudflare's nameservers yet.
- *too many certificates* → Let's Encrypt's weekly limit (5 identical certs). Wait, and
  use the staging CA while testing.

See [tls.md](tls.md) for the full setup.

---

## Docker step is red / "Docker is installed but not running"

- Start your Docker engine and wait until it's running, then click **Check**. (Docker Desktop: open it, wait for "running". Colima: `colima start`.)
- Linux: `sudo systemctl start docker`; make sure your user is in the `docker` group (`sudo usermod -aG docker $USER`, then re-login).
- "Compose plugin is missing": install `docker compose` v2 — bundled with Docker Desktop, but a separate package (e.g. `docker-compose-plugin`) on some Docker Engine / Colima setups.

---

## File uploads or image previews fail (but the rest works)

**Cause:** the presigned file URLs point at an address the device can't reach, or at
`http://` on an HTTPS page (blocked as mixed content).

- `BUCKET_EXTERNAL_ENDPOINT` is set automatically from `CARE_PUBLIC_HOST` — don't edit
  it in `backend.env`. If it's wrong, fix the address in `tls.env` and run `care start`.
- If the clinic's address changed recently, the frontend may still be built for the old
  one: `care rebuild-frontend`.
- Check `care status` shows `minio running` (Caddy proxies to it).

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
