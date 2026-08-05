# Install CARE Desktop on Windows

Follow these once on the **server PC** — the computer that stays on and runs the
clinic. Other devices install nothing; they open the clinic's web address.

> Budget ~15–25 minutes for the first setup. Internet is needed **for setup only**.
> The app is pure Go — **no WSL, no Git Bash, no bash** is required to run it.

---

## 1. Requirements

| Need | Why | How |
|---|---|---|
| **Windows 10/11** (64-bit) | the server OS | — |
| **Docker**, running (WSL 2 backend, + `docker compose` v2) | runs the whole stack | Docker Desktop is simplest ([docker.com](https://www.docker.com/products/docker-desktop/) → install → enable WSL 2 → open). Rancher Desktop, or Docker Engine inside WSL 2, also work. |
| **Git for Windows** | downloads + builds CARE once | [git-scm.com/download/win](https://git-scm.com/download/win) → install with defaults |
| **A domain on Cloudflare** | so every device gets HTTPS with nothing installed (see step 2) | any registrar + the free Cloudflare plan — see [tls.md](tls.md) |

> **Hardware:** 8 GB RAM minimum (16 GB recommended), ~10 GB free disk. Docker
> Desktop on Windows needs virtualization enabled in the BIOS (usually on by default).

---

## 2. Set up the clinic's web address

CARE is served over HTTPS at a domain you own — that's what gives every phone and
laptop a working padlock with **nothing to install on them**. Setup won't run without
it, because there is no plain-HTTP mode.

You need, once:

1. **A domain** (any registrar, ~₹900/year)
2. **Its DNS moved to Cloudflare** (free plan — you keep the domain where you bought it)
3. **An `A` record** — e.g. `clinic.yourdomain.com` → this PC's address on the clinic
   WiFi, with Cloudflare's proxy **off** (grey cloud)
4. **A Cloudflare API token** scoped to that domain ("Edit zone DNS" template)
5. **A DHCP reservation** on the clinic router so this PC keeps the same address

Find this PC's address with `ipconfig` (look for *IPv4 Address* on the adapter that's
on the clinic WiFi). Full walkthrough: **[tls.md](tls.md)**.

> **Good news for Windows specifically:** the old setup needed mDNS, Apple Bonjour, and
> a firewall rule for UDP 5353 to make `care.local` resolve — the fiddliest part of a
> Windows install. None of that applies any more. The address is ordinary public DNS,
> which Windows and every client device handle natively.

You will still need to allow **inbound TCP 443** if Windows Firewall prompts on first
start, so devices on the WiFi can reach the server.

## 3. Get the app

1. Download `CARE-Desktop-windows.zip` from the project's **GitHub Releases** page.
2. Unzip it. (Windows SmartScreen may warn about an unsigned app — **More info → Run anyway**.)
3. Run **CARE Desktop.exe**.

---

## 4. Run the setup wizard

The installer shows gated steps — each must be green before **Install & Start** enables:

1. **Docker** — green when your Docker engine is running.
2. **Git** — green when Git for Windows is installed.
3. **Clinic web address** — enter your domain and Cloudflare token, then **Check address**. It verifies the token with Cloudflare and that the record points at this PC.
4. **Install location** *(optional)*.
5. **Backup location** *(optional)* — a USB/external drive is recommended.
6. **Admin password** *(optional)* — blank = `admin`.

Click **Install & Start**. It clones + builds CARE and starts the stack (several minutes).

---

## 5. Log in

Open **https://clinic.yourdomain.com/** on any device on the WiFi:

- **Username:** `admin`
- **Password:** what you set (or `admin`)

**Change it immediately** at `/admin/`.

---

## Notes

- Tick **Start at login** in the panel so CARE returns after a reboot. Also set your
  Docker engine to start at login (Docker Desktop: **Settings → General → Start
  Docker Desktop when you log in**), so the containers come back automatically.
- Closing the window leaves CARE running.
- See [troubleshooting.md](troubleshooting.md) for certificate and Docker issues.
