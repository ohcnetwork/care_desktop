# Install CARE Desktop on macOS

Follow these once on the **server Mac** — the computer that stays on and runs the
clinic. Other devices (phones, laptops) install nothing; they just open
`https://care.local`.

> Budget ~15–20 minutes for the first setup (it downloads + builds CARE). You need
> internet **for the setup only**; after that it runs offline.

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

## 2. Name the Mac `care` (so devices find it)

Devices reach the clinic at `https://care.local`. macOS advertises this via Bonjour
once the Mac's **Local Hostname** is `care`.

Run in **Terminal** (it asks for your password):
```bash
sudo scutil --set LocalHostName care
```
Or: **System Settings → General → Sharing → Local hostname** → set to `care`.

Verify:
```bash
scutil --get LocalHostName     # should print: care
```

> **This step is for *other* devices.** The Mac running CARE resolves `care.local`
> through the hosts entry setup adds (`127.0.0.1 care.local`, one admin prompt), not
> through Bonjour. A Mac's own resolver won't return a name that a second responder
> on the same machine advertises. If you declined that prompt, the server's own
> browser can't open the clinic; add it by hand with
> `echo "127.0.0.1 care.local" | sudo tee -a /etc/hosts`.

> The desktop installer also **checks** this for you (step 3) and shows these exact
> instructions if it isn't set — but a GUI can't ask for your password, so set it in
> Terminal once as above.

---

## 3. Get the app

**Option A — Desktop app (recommended):**
1. Download `CARE-Desktop-<version>-macos.dmg` from the project's **GitHub Releases** page.
2. Open the `.dmg` and drag **CARE Desktop** onto the **Applications** shortcut beside it.
3. First open: right-click → **Open** (to bypass the unsigned-app warning), then **Open** again.

> The `.dmg` is universal — one download for both Apple Silicon and Intel.

**Option B — Command line** (for developers): see [building.md](building.md) to build
the `care` CLI, then jump to [cli.md](cli.md).

---

## 4. Run the setup wizard

Open **CARE Desktop**. The installer shows gated steps — each must be green:

1. **Docker** — green when your Docker engine is running. (If red: start Docker, click **Check**.)
2. **Git** — green when git is installed.
3. **Network name — care.local** — green when step 2 above is done.
4. **Install location** *(optional)* — where the app's files go. Leave default if unsure.
5. **Backup location** *(optional)* — pick a **USB/external drive** if you have one (recommended).
6. **Admin password** *(optional)* — the first login's password; blank = `admin`.

When 1–3 are green, click **Install & Start**. It clones + builds CARE and brings the
stack up. Watch the log; the first run takes several minutes.

---

## 5. Log in

When it finishes, open **https://care.local/** (on the Mac or any device on the same
WiFi) and log in:

- **Username:** `admin`
- **Password:** what you set in step 6 (or `admin`)

**Change the password immediately** at `https://care.local/admin/`.

## 6. Trust the certificate on client devices

The clinic runs over HTTPS with a self-signed cert. **This Mac (the server) trusts it
automatically** the first time the stack starts — you may see a one-time admin prompt
to add it to the keychain; approve it. **Every other device** trusts it once:

1. On the device, open **`https://care.local/setup`** (type the full `https://`).
2. Pick the device type and follow the two steps: download `root.crt`, then add it to
   the system trust store. It appears as **"CARE Desktop Local CA"**.
3. **iOS:** install the profile in **Safari**, then enable it under **Settings →
   General → About → Certificate Trust Settings**. Both steps are required.

After trusting, the padlock turns green and the camera/file features work. See
[troubleshooting.md](troubleshooting.md#security-warning--red-padlock-instead-of-a-green-one)
if a device still shows a warning.

---

## Day-to-day

- The app's **Start / Stop / Restart / Rebuild / Backup now** buttons control everything.
- Tick **Start at login** so CARE comes up automatically after a reboot.
- Closing the window leaves CARE **running** (it's the Docker stack, not the window).

See [cli.md](cli.md) for the terminal equivalents, [configuration.md](configuration.md)
to change settings, and [troubleshooting.md](troubleshooting.md) if something's off.
