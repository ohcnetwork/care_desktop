# Install CARE Desktop on macOS

Follow these once on the **server Mac** — the computer that stays on and runs the
clinic. Other devices (phones, laptops) install nothing; they just open the clinic's
web address in a browser.

> Budget ~15–20 minutes for the first setup (it downloads + builds CARE). You need
> internet for setup, and afterwards only to renew the certificate every couple of
> months.

---

## 1. Requirements

| Need | Why | How |
|---|---|---|
| **macOS** 12+ (Apple Silicon or Intel) | the server OS | — |
| **Docker**, running (+ `docker compose` v2) | runs the whole stack | Docker Desktop is the simplest ([docker.com](https://www.docker.com/products/docker-desktop/) → install → open → wait for "running"). Colima (`brew install colima docker docker-compose && colima start`), OrbStack, or Podman also work. |
| **Git** | downloads + builds CARE once | `git --version` — if missing, it prompts to install the Command Line Tools, or run `xcode-select --install` |

> **Hardware:** any Mac that can run Docker comfortably. 8 GB RAM minimum,
> 16 GB recommended. ~10 GB free disk for the images + data.

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

**Option A — Desktop app (recommended):**
1. Download `CARE-Desktop-macos.zip` from the project's **GitHub Releases** page.
2. Unzip it and move **CARE Desktop.app** to **Applications**.
3. First open: right-click → **Open** (to bypass the unsigned-app warning), then **Open** again.

**Option B — Command line** (for developers): see [building.md](building.md) to build
the `care` CLI, then jump to [cli.md](cli.md).

---

## 4. Run the setup wizard

Open **CARE Desktop**. The installer shows gated steps — each must be green:

1. **Docker** — green when your Docker engine is running. (If red: start Docker, click **Check**.)
2. **Git** — green when git is installed.
3. **Clinic web address** — enter your domain and Cloudflare token, then **Check address**. It verifies the token with Cloudflare and that the record points at this Mac.
4. **Install location** *(optional)* — where the app's files go. Leave default if unsure.
5. **Backup location** *(optional)* — pick a **USB/external drive** if you have one (recommended).
6. **Admin password** *(optional)* — the first login's password; blank = `admin`.

When 1–3 are green, click **Install & Start**. It clones + builds CARE and brings the
stack up. Watch the log; the first run takes several minutes.

---

## 5. Log in

When it finishes, open **https://clinic.yourdomain.com/** (on the Mac or any device on the same
WiFi) and log in:

- **Username:** `admin`
- **Password:** what you set in step 6 (or `admin`)

**Change the password immediately** at `https://clinic.yourdomain.com/admin/`.

---

## Day-to-day

- The app's **Start / Stop / Restart / Rebuild / Backup now** buttons control everything.
- Tick **Start at login** so CARE comes up automatically after a reboot.
- Closing the window leaves CARE **running** (it's the Docker stack, not the window).

See [cli.md](cli.md) for the terminal equivalents, [configuration.md](configuration.md)
to change settings, and [troubleshooting.md](troubleshooting.md) if something's off.
