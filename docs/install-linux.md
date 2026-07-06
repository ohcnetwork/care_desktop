# Install CARE Desktop on Linux

Follow these once on the **server machine** — the computer that stays on and runs
the clinic. Other devices install nothing; they open `http://care.local`.

> Budget ~15–20 minutes for the first setup. Internet is needed **for setup only**.

---

## 1. Requirements

| Need | Why | How |
|---|---|---|
| A 64-bit Linux (Ubuntu/Debian/Fedora, etc.) | the server OS | — |
| **Docker Engine** + **docker compose v2**, running | runs the whole stack | [docs.docker.com/engine/install](https://docs.docker.com/engine/install/) — install Docker Engine + the Compose plugin |
| **git** | downloads + builds CARE once | `sudo apt install git` / `sudo dnf install git` |
| **Avahi** | advertises `care.local` | `sudo apt install avahi-daemon` / `sudo dnf install avahi` (the CLI/app sets this up too) |
| **WebKitGTK** (only for the desktop app) | renders the app window | `sudo apt install libgtk-3-0 libwebkit2gtk-4.0-37` |

> Add your user to the `docker` group so you don't need `sudo` for Docker:
> `sudo usermod -aG docker $USER` then log out/in.

> **Hardware:** 8 GB RAM minimum (16 GB recommended), ~10 GB free disk.

---

## 2. Name the machine `care`

Devices reach the clinic at `http://care.local`, advertised by Avahi once the
hostname is `care`:

```bash
sudo hostnamectl set-hostname care
sudo systemctl enable --now avahi-daemon
```

Verify:
```bash
hostname        # should print: care
```

> The desktop installer **checks** this (step 3) and shows these instructions if it
> isn't set. A GUI can't prompt for sudo, so run the commands above once in a terminal
> (or run the CLI `care setup`, which does it for you with sudo).

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
3. **Network name — care.local** — green once the hostname is `care` (step 2).
4. **Install location** *(optional)*.
5. **Backup location** *(optional)* — a separate/USB drive is recommended.
6. **Admin password** *(optional)* — blank = `admin`.

Click **Install & Start**. It clones + builds CARE and starts the stack (several minutes).

---

## 5. Log in

Open **http://care.local/** on any device on the WiFi:

- **Username:** `admin`
- **Password:** what you set (or `admin`)

**Change it immediately** at `http://care.local/admin/`.

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
