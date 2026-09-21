# Cannot open CARE from a computer that previously hosted a clinic

Use this guide when a Windows, macOS, or Linux computer **previously ran CARE
Desktop as a server**, but now needs to connect to a different CARE server.
Other devices may open the clinic normally while this computer times out.

This is a recovery procedure for an earlier installation, not a setup
requirement for every client. The saved Server/Client role is not an ordinary
switch: do not delete configuration or clinic data merely to change it.

For an ordinary CARE Desktop client leaving a clinic, use **Uninstall client
setup**, not the server-uninstall procedures below. It removes only that
client's saved connection and certificate it installed, preserving all server data.
Successful uninstall clears the role so Server or Client can be selected again.
If you also want to remove the desktop executable, uninstall it through the
operating system afterwards. Pre-existing trusted certificates are intentionally
preserved and may still allow browser access.
See [client removal](native-integrations.md#removing-client-access).

## Why this happens

During local server setup, CARE can add a line such as:

```text
127.0.0.1 care.local # care-desktop
```

That makes this computer resolve `care.local` to **itself**, rather than discover
the real server on the clinic network. The entry can remain even when Docker is
not running or CARE Desktop has been deleted. Reinstalling a certificate or
rebuilding the frontend will not fix that address override.

The app's explicit **Uninstall** and **Remove the earlier CARE Desktop** cleanup
paths remove CARE-owned hosts entries. Merely stopping the clinic, quitting, or
deleting the application does not guarantee their removal. See
[#13](https://github.com/ohcnetwork/care_desktop/issues/13).

## First confirm the stale entry

Run these checks on the **affected former server/client computer**, not on the
computer currently hosting the clinic. Replace `care.local` in this guide with
your clinic's actual name.

| System | Hosts file |
| --- | --- |
| Windows | `%WINDIR%\System32\drivers\etc\hosts` |
| macOS / Linux | `/etc/hosts` |

On Windows, inspect CARE-marked lines in PowerShell:

```powershell
Select-String -LiteralPath "$env:WINDIR\System32\drivers\etc\hosts" -Pattern '# care-desktop' -SimpleMatch
```

On macOS or Linux:

```sh
grep -nF '# care-desktop' /etc/hosts
```

Look for a CARE-marked loopback entry for the hostname you are trying to open.
If there is no such entry, do not run a full uninstall to troubleshoot this
symptom. Check network connectivity, mDNS, and certificate trust separately.
Unmarked entries may have been added manually; have their owner review them.

## Smallest repair: keep the old installation and its data

If you only need to remove the address override:

1. Quit CARE Desktop on this former server. Starting its old clinic again can
   recreate the local hosts entry.
2. Make a backup copy of its hosts file before editing.
3. On Windows, open Notepad **as Administrator**, then open the hosts file
   (choose **All Files** in the file picker). On macOS/Linux, open it in an
   administrator-authorized editor, for example `sudo nano /etc/hosts`.
4. Remove only the CARE-marked loopback line for the affected clinic hostname.
   Preserve unrelated mappings and comments, and save the file.
5. On Windows, run `ipconfig /flushdns`. On macOS, run
   `sudo dscacheutil -flushcache`. On Linux using systemd-resolved, run
   `sudo resolvectl flush-caches`; otherwise use that system's resolver-specific
   cache flush or restart the computer.
6. Fully close and reopen the browser, then visit `https://care.local/`.

This repair does not remove Docker data, backups, certificates, or the
application. Do not remove the current CARE server's intentional loopback
mapping as a routine client troubleshooting step.

## Safety net: standalone cleanup of an unwanted earlier installation

If the earlier installation is no longer needed and the app cannot open, is
already gone, or its removal did not finish, the standalone scripts provide a
fallback to the app's cleanup flows:

- [Windows cleanup script](../uninstall-windows.ps1)
- [macOS cleanup script](../uninstall-macos.sh)

**These are full-uninstall scripts, not hosts-only repairs. They delete the old
clinic's live database/file-storage volumes, containers, installed files, and
settings. They also remove CARE certificate trust and other local integration.**
Keeping the backup folder is not the same as keeping the live clinic data.

Before proceeding, confirm this is not the active clinic server and preserve a
current, verified backup and its recovery material outside the installation.
Download the appropriate script as a file and review it; do not pipe a download
directly into a shell. Run it as the normal user who installed CARE, from the
folder containing the downloaded script. The Docker engine (Rancher Desktop on
macOS and Windows) must be running if
Docker resources are to be inspected and removed.

### Windows

Open a normal PowerShell window, **not** an administrator window. First inspect
what would be removed:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\uninstall-windows.ps1 -DryRun
```

Only after reviewing that output and deciding to delete the old installation:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\uninstall-windows.ps1
```

The script asks for confirmation and elevates the system-cleanup steps when
needed. Read any reported failures; an administrator prompt alone is not proof
that cleanup succeeded.

### macOS

Run from Terminal as your normal user, **without prefixing the script with
`sudo`**. First inspect:

```sh
bash ./uninstall-macos.sh --dry-run
```

Only after reviewing that output and deciding to delete the old installation:

```sh
bash ./uninstall-macos.sh
```

The script requests administrator approval for the steps that need it. Read any
reported failures before assuming the computer is clean.

### What to keep and what to do afterwards

The scripts retain backups by default and attempt to preserve their recovery
key. **Do not add `-RemoveBackups` / `--remove-backups` for client recovery.** Do
not use `-Yes` / `--yes` to bypass the confirmation. There is no standalone
Linux cleanup script in this repository; use the hosts-only procedure above or
the app's explicit uninstall flow.

After cleanup, repeat the hosts-file check and flush the resolver cache as
described above. Open the clinic URL again. On Windows,
`Test-NetConnection care.local -Port 443` can show the resolved address and
whether HTTPS is reachable: it should target the real server, not `127.0.0.1`
or `::1`.

Because full cleanup removes CARE certificates, use CARE Desktop's native
client setup to trust the **current server's** certificate. Enter the `.local`
address shown on that server, approve the operating-system prompt if requested,
and let CARE verify HTTPS automatically. Use a trusted clinic network: the
initial HTTP certificate download is trust on first use, not independent proof
of the server's identity. Do not bypass browser certificate warnings.

If the hostname still cannot be found or the connection still times out, do not
keep rerunning the cleanup script. There may be a separate Wi-Fi, mDNS, routing,
or server problem. See [client/network limits](native-integrations.md#client-independence-and-network-limits).
