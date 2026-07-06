# Install CARE Desktop on macOS

Follow these once on the **server Mac** — the computer that stays on and runs the
clinic. Other devices (phones, laptops) install nothing; they just open
`http://care.local`.

> Budget ~15–20 minutes for the first setup (it downloads + builds CARE). You need
> internet **for the setup only**; after that it runs offline.

---

## 1. Requirements

| Need | Why | How |
|---|---|---|
| **macOS** 12+ (Apple Silicon or Intel) | the server OS | — |
| **Docker Desktop**, running | runs the whole stack | [docker.com/products/docker-desktop](https://www.docker.com/products/docker-desktop/) → install → open it → wait for "Docker Desktop is running" |
| **Git** | downloads + builds CARE once | `git --version` — if missing, it prompts to install the Command Line Tools, or run `xcode-select --install` |

> **Hardware:** any Mac that can run Docker Desktop comfortably. 8 GB RAM minimum,
> 16 GB recommended. ~10 GB free disk for the images + data.

---

## 2. Name the Mac `care` (so devices find it)

Devices reach the clinic at `http://care.local`. macOS advertises this via Bonjour
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

> The desktop installer also **checks** this for you (step 3) and shows these exact
> instructions if it isn't set — but a GUI can't ask for your password, so set it in
> Terminal once as above.

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

1. **Docker** — green when Docker Desktop is running. (If red: start Docker, click **Check**.)
2. **Git** — green when git is installed.
3. **Network name — care.local** — green when step 2 above is done.
4. **Install location** *(optional)* — where the app's files go. Leave default if unsure.
5. **Backup location** *(optional)* — pick a **USB/external drive** if you have one (recommended).
6. **Admin password** *(optional)* — the first login's password; blank = `admin`.

When 1–3 are green, click **Install & Start**. It clones + builds CARE and brings the
stack up. Watch the log; the first run takes several minutes.

---

## 5. Log in

When it finishes, open **http://care.local/** (on the Mac or any device on the same
WiFi) and log in:

- **Username:** `admin`
- **Password:** what you set in step 6 (or `admin`)

**Change the password immediately** at `http://care.local/admin/`.

---

## Day-to-day

- The app's **Start / Stop / Restart / Rebuild / Backup now** buttons control everything.
- Tick **Start at login** so CARE comes up automatically after a reboot.
- Closing the window leaves CARE **running** (it's the Docker stack, not the window).

See [cli.md](cli.md) for the terminal equivalents, [configuration.md](configuration.md)
to change settings, and [troubleshooting.md](troubleshooting.md) if something's off.
