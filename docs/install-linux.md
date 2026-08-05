# Install CARE Desktop on Linux

Follow these once on the **server machine** — the computer that stays on and runs
the clinic. Other devices install nothing; they open the clinic's web address.

> Budget ~15–20 minutes for the first setup. Internet is needed **for setup only**.

---

## 1. Requirements

| Need | Why | How |
|---|---|---|
| A 64-bit Linux (Ubuntu/Debian/Fedora, etc.) | the server OS | — |
| **Docker Engine** + **docker compose v2**, running | runs the whole stack | [docs.docker.com/engine/install](https://docs.docker.com/engine/install/) — install Docker Engine + the Compose plugin |
| **git** | downloads + builds CARE once | `sudo apt install git` / `sudo dnf install git` |
| **WebKitGTK** (only for the desktop app) | renders the app window | `sudo apt install libgtk-3-0 libwebkit2gtk-4.0-37` |

> Add your user to the `docker` group so you don't need `sudo` for Docker:
> `sudo usermod -aG docker $USER` then log out/in.

> **Hardware:** 8 GB RAM minimum (16 GB recommended), ~10 GB free disk.

---

## 2. Set up the clinic's web address

CARE is served over HTTPS at a domain you own — that's what gives every phone and
laptop a working padlock with **nothing to install on them**. Setup won't run without
it, because there is no plain-HTTP mode.

You need, once:

1. **A domain** (any registrar, ~₹900/year)
2. **Its DNS moved to Cloudflare** (free plan — you keep the domain where you bought it)
3. **An `A` record** — e.g. `clinic.yourdomain.com` → this computer's address on the
   clinic WiFi, with Cloudflare's proxy **off** (grey cloud)
4. **A Cloudflare API token** scoped to that domain ("Edit zone DNS" template)
5. **A DHCP reservation** on the clinic router so this computer keeps the same address

Full walkthrough with screenshots-worth of detail: **[tls.md](tls.md)**.

> The address is public, but it points at a private address that only means something
> inside the building — so traffic never crosses the internet. The server goes online
> only to renew the certificate, about every two months.

---

## 3. Get the app

**Option A — Desktop app:**
1. Download `CARE-Desktop-linux.tar.gz` from the project's **GitHub Releases** page.
2. Extract it: `tar -xzf CARE-Desktop-linux.tar.gz`
3. Run the **CARE Desktop** binary (`./CARE\ Clinic`). Mark it executable if needed: `chmod +x`.

**Option B — Command line:** build the `care` CLI ([building.md](building.md)) or run
it directly, then see [cli.md](cli.md):
```bash
cd care-desktop
go run ./app/cmd/care setup     # then: ... start
# (run from the repo root so it finds docker-compose.yml)
```

---

## 4. Run the setup wizard (desktop app)

Each gated step must be green:

1. **Docker** — green when the Docker daemon is reachable.
2. **Git** — green when git is installed.
3. **Clinic web address** — enter your domain and Cloudflare token, then **Check address**.
4. **Install location** *(optional)*.
5. **Backup location** *(optional)* — a separate/USB drive is recommended.
6. **Admin password** *(optional)* — blank = `admin`.

Click **Install & Start**. It clones + builds CARE and starts the stack (several minutes).

---

## 5. Log in

Open **https://clinic.yourdomain.com/** on any device on the WiFi:

- **Username:** `admin`
- **Password:** what you set (or `admin`)

**Change it immediately** at `https://clinic.yourdomain.com/admin/`.

---

## Run it headless (no desktop)

On a server with no GUI, skip the desktop app entirely and use the CLI:
```bash
cd care-desktop
go run ./app/cmd/care setup
go run ./app/cmd/care start
```
To start CARE on boot, the containers already have `restart: unless-stopped`, so once
Docker starts at boot the stack returns. (Enable Docker at boot:
`sudo systemctl enable docker`.)

See [cli.md](cli.md) for all commands and [troubleshooting.md](troubleshooting.md) for fixes.
