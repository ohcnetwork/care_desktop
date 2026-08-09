# Install CARE Desktop on Windows

Follow these once on the **server PC** — the computer that stays on and runs the
clinic. Other devices install nothing; they open `https://care.local`.

> Budget ~15–25 minutes for the first setup. Internet is needed **for setup only**.
> The app is pure Go — **no WSL, no Git Bash, no bash** is required to run it.

---

## 1. Requirements

| Need | Why | How |
|---|---|---|
| **Windows 10/11** (64-bit) | the server OS | — |
| **Docker**, running (WSL 2 backend, + `docker compose` v2) | runs the whole stack | Docker Desktop is simplest ([docker.com](https://www.docker.com/products/docker-desktop/) → install → enable WSL 2 → open). Rancher Desktop, or Docker Engine inside WSL 2, also work. |
| **Git for Windows** | downloads + builds CARE once | [git-scm.com/download/win](https://git-scm.com/download/win) → install with defaults |
| Working **mDNS** for `care.local` | so *other devices* find the server by name (see step 2) | native on recent Windows 11; otherwise [Apple Bonjour](https://support.apple.com/kb/DL999) or a static IP. *The server's own browser needs nothing — CARE adds a hosts entry automatically.* |

> **Hardware:** 8 GB RAM minimum (16 GB recommended), ~10 GB free disk. Docker
> Desktop on Windows needs virtualization enabled in the BIOS (usually on by default).

---

## 2. Make `care.local` resolvable

Everything is reached at `https://care.local`. There are two audiences, and they're
handled differently:

- **The server's own browser — automatic, nothing to do.** During setup CARE adds a
  single hosts-file line, `127.0.0.1  care.local` (via a one-time UAC prompt), so the
  server machine resolves `care.local` to itself. **No PC rename, no Bonjour.** The
  line is tagged `# care-desktop` and is removed again on uninstall. Just open
  **`https://care.local`** on the server.
- **Other devices on the WiFi** (phones, staff laptops) find the server through
  CARE's built-in mDNS responder. Windows can resolve `.local` names natively, and the
  in-app responder advertises `care.local` regardless of the PC's name — so this
  usually works once the **two network settings below** are right. The installer's
  **step-3 Check** tests whether `care.local` resolves, so you never have to guess.

**Why these settings?** Windows blocks device-discovery (mDNS) by default in two
places: on **Public** networks, and at the **firewall**. You're unblocking both so
the responder's announcements reach other devices. *(Renaming the PC to `care` is
**not** required — the responder handles the name itself.)*

> **Fastest path:** open **PowerShell as Administrator** (right-click Start →
> **Terminal (Admin)**) and run these two:
> ```powershell
> Set-NetConnectionProfile -NetworkCategory Private
> New-NetFirewallRule -DisplayName "mDNS (care.local)" -Direction Inbound -Protocol UDP -LocalPort 5353 -Action Allow -Profile Private
> ```
> Then run `ping care.local` to confirm, and click **Check** in the app.

If you prefer clicking through the menus, do the same two steps below.

### Step 1 — Set the network to Private
- **Settings** (Win + I) → **Network & Internet** → click your **Wi-Fi**/**Ethernet** → click the **network name** → under **Network profile type**, choose **Private**.
- PowerShell: `Set-NetConnectionProfile -NetworkCategory Private`
- *Why:* Windows hides the PC and blocks mDNS on Public networks; Private allows discovery.

### Step 2 — Allow mDNS (UDP 5353) in the firewall
Usually already allowed once the network is Private — add this only if devices still can't find the PC.
- **Windows Defender Firewall with Advanced Security** → **Inbound Rules** → **New Rule…** → **Port** → **UDP**, port `5353` → **Allow the connection** → tick **Private** → name it `mDNS (care.local)` → **Finish**.
- PowerShell: `New-NetFirewallRule -DisplayName "mDNS (care.local)" -Direction Inbound -Protocol UDP -LocalPort 5353 -Action Allow -Profile Private`
- *Why:* mDNS announcements travel on UDP port 5353; the firewall must let them through.

### Step 3 — Verify
- On the server: open `https://care.local/` → the login page loads (works immediately, thanks to the hosts entry). `ping care.local` also replies.
- In the CARE Desktop app: click **Check** on step 3 → it turns green.
- From a phone on the same WiFi: open `https://care.local/` → the login page loads.

### If other devices still can't reach it — pick one:

**Option A — Apple Bonjour** (reliable `care.local` on any Windows version):
- Install [Bonjour Print Services](https://support.apple.com/kb/DL999) on the server, re-check. (No PC rename needed.)

**Option B — Static IP** (no extra software, no `.local`):
- Give the PC a fixed IP (router DHCP reservation), e.g. `192.168.1.50`; staff open `https://192.168.1.50/`.
- Also set `BUCKET_EXTERNAL_ENDPOINT=https://192.168.1.50` in `backend.env`, and
  `REACT_CARE_API_URL=https://192.168.1.50` in `frontend.env` (then `care rebuild-frontend`).
- Since the frontend is built for `care.local` by default, **Option A is smoother** — use the static IP only if mDNS is blocked on your network.

> **Client devices need nothing installed.** Macs, iPhones, and Linux resolve `care.local` out
> of the box; modern Android usually does too (older Android may need the IP).

---

## 3. Get the app

1. Download `CARE-Desktop-windows.zip` from the project's **GitHub Releases** page.
2. Unzip it. (Windows SmartScreen may warn about an unsigned app — **More info → Run anyway**.)
3. Run **CARE Desktop.exe**.

---

## 4. Run the setup wizard

The installer shows gated steps — each must be green before **Install & Start** enables:

1. **Docker** — green when your Docker engine is running.
2. **Git** — green when Git for Windows is installed.
3. **Network name — care.local** — green once `care.local` resolves (native on recent Windows 11 with a Private network + UDP 5353 allowed; otherwise via Bonjour — see step 2).
4. **Install location** *(optional)*.
5. **Backup location** *(optional)* — a USB/external drive is recommended.
6. **Admin password** *(optional)* — blank = `admin`.

Click **Install & Start**. It clones + builds CARE and starts the stack (several minutes).

---

## 5. Log in

Open **https://care.local/** (or `https://<your-static-ip>/`) on any device on the WiFi:

- **Username:** `admin`
- **Password:** what you set (or `admin`)

**Change it immediately** at `https://care.local/admin/`.

## Trust the certificate on client devices

The clinic runs over HTTPS with a self-signed cert. **This PC (the server) trusts it
automatically** on first start — approve the one-time UAC prompt to add it to the
Windows certificate store. **Every other device** trusts it once by opening
**`https://care.local/setup`**, picking the device, and following the two steps
(download `root.crt` → add to the trust store). It appears as **"CARE Desktop Local
CA"**. On **iOS**, install the profile in **Safari**, then enable it under **Settings →
General → About → Certificate Trust Settings**. See
[troubleshooting.md](troubleshooting.md#security-warning--red-padlock-instead-of-a-green-one)
if a warning persists.

---

## Notes

- Tick **Start at login** in the panel so CARE returns after a reboot. Also set your
  Docker engine to start at login (Docker Desktop: **Settings → General → Start
  Docker Desktop when you log in**), so the containers come back automatically.
- Closing the window leaves CARE running.
- See [troubleshooting.md](troubleshooting.md) for `care.local` and Docker issues.
