# Your clinic's web address (HTTPS)

CARE is served at `https://<your domain>` — a real padlock, with a certificate every
browser already trusts. **Clinic devices install nothing.** The server obtains its own
certificate and renews it forever without anyone doing anything.

This is not optional, and there is no plain-HTTP mode. On a WiFi network shared with
anyone else, unencrypted traffic exposes logins and patient records to everyone on it —
and a plaintext entry point can be intercepted and replaced with a fake login page, so
even "redirect to HTTPS" isn't safe enough to offer.

---

## What you need

| | |
|---|---|
| A domain | Any registrar (Namecheap, GoDaddy…). About ₹900/year. |
| Cloudflare account | Free plan. It hosts the domain's DNS. |
| Internet on the server | Setup, then renewal every ~60 days. Never for day-to-day use. |

> **Why Cloudflare specifically?** The server proves it owns the domain by writing a
> DNS record automatically, which needs a provider with a usable API. Cloudflare is the
> one supported path, so there is one setup to document and one integration that's
> actually tested. Buy the domain wherever you like — only the DNS hosting moves.

**Internet is needed for renewal, not for use.** Certificates last 90 days and renewal
starts at day 60, so the connection would have to be down for a **month** before a
nurse noticed anything. Day-to-day traffic never leaves the building.

---

## The counterintuitive part

Your domain will point at a **private** address like `192.168.1.50`.

That looks wrong, but it's the whole trick. `192.168.1.50` only means something inside
the clinic building. Someone elsewhere who looks up your name is told "it's at
192.168.1.50", their device tries it, and reaches their own network — never yours.

- **Public:** the name, and the paperwork proving the name is yours
- **Private:** the actual connection — device → WiFi → server, never leaves the room

You borrow the internet's *trust system* without using the internet as a *pipe*.

---

## Step 1 — Find the server's address

You only need this for the initial record; **CARE keeps it up to date afterwards by
itself** (see [Moving networks](#moving-networks-and-changing-addresses) below).

```sh
ipconfig getifaddr en0     # macOS
hostname -I                # Linux
ipconfig                   # Windows — the IPv4 address on the clinic WiFi
```

It looks like `192.168.1.50`.

> Optional, but good practice: add a **DHCP reservation** on the clinic router so this
> computer always gets the same address. Not required — the server updates its own
> record — but it means one less thing moving.

## Step 2 — Move the domain's DNS to Cloudflare

You keep the domain where you bought it. Only the nameservers change.

1. Sign up at [cloudflare.com](https://cloudflare.com), **Add a site**, enter your
   domain, pick the **Free** plan.
2. Cloudflare shows two nameservers, e.g. `dana.ns.cloudflare.com`.
3. At your registrar, replace the existing nameservers with those two.
   - *Namecheap:* Domain List → Manage → Nameservers → **Custom DNS**
4. Wait until Cloudflare shows the domain as **Active** — usually minutes, sometimes a
   few hours. Verify with `dig +short NS yourdomain.com`.

Nothing works until this shows Active: certificate issuance depends on Cloudflare being
authoritative for your domain.

## Step 3 — Point a name at the server

Cloudflare → your domain → **DNS** → **Add record**:

| Field | Value |
|---|---|
| Type | `A` |
| Name | `clinic` (giving `clinic.yourdomain.com`) |
| IPv4 address | the server's LAN IP from Step 1, e.g. `192.168.1.50` |
| Proxy status | **DNS only** — grey cloud, not orange |
| TTL | **1 min** while setting up |

> **The grey cloud is not optional.** Orange means Cloudflare proxies traffic through
> its own network, which cannot work for a private address. If you enter a private IP
> Cloudflare usually forces *DNS only* by itself and labels it "reserved IP" — that's
> the state you want.

Verify: `dig +short clinic.yourdomain.com @1.1.1.1` should print your server's LAN IP.

## Step 4 — Create an API token

The server uses this to prove it owns the domain at every renewal.

1. Cloudflare → **My Profile** → **API Tokens** → **Create Token**
2. Use the **Edit zone DNS** template
3. *Zone Resources*: **Include → Specific zone → yourdomain.com**
4. Create it and **copy the token now** — Cloudflare shows it once

Scoping to the one zone matters: this token lives on the clinic server, so if that
machine is stolen you want the damage limited to a single domain.

Check it works:

```sh
curl -s -H "Authorization: Bearer YOUR_TOKEN" \
  https://api.cloudflare.com/client/v4/user/tokens/verify
```

Look for `"status":"active"`.

## Step 5 — Install

**Desktop app:** the setup wizard's **Clinic web address** section takes the domain and
the token, and **Check address** verifies all three things before you commit: the name
is shaped right, Cloudflare accepts the token, and the record already points at this
computer.

**CLI:** edit `tls.env` in the kit folder:

```sh
CARE_PUBLIC_HOST=clinic.yourdomain.com
CLOUDFLARE_API_TOKEN=<the token, with nothing around it>
```

then `care setup && care start` (or just `care start` if already set up).

The first run obtains the certificate and builds the frontend against your address —
the app's API URL is compiled in at build time, so this is a rebuild, not a restart.

### Test with staging first

Let's Encrypt allows only **5 identical certificates per week**. Failed validations
reset hourly, but repeated *successful* issuance can lock you out for seven days.
Before the real run, set:

```sh
CARE_ACME_CA=https://acme-staging-v02.api.letsencrypt.org/directory
```

Staging certificates are deliberately **not** trusted by browsers — you'll still see a
warning page. That's the expected result and it proves DNS, the token, and the whole
exchange work. Restore the production URL and run `care start` again for the real one.

---

## Check it worked

```sh
curl -sI https://clinic.yourdomain.com/ping/ | head -1     # HTTP/2 200
echo | openssl s_client -connect clinic.yourdomain.com:443 \
  -servername clinic.yourdomain.com 2>/dev/null \
  | openssl x509 -noout -issuer -subject -dates
```

Issuer should be Let's Encrypt, subject your domain, validity ~90 days.

Then open `https://clinic.yourdomain.com` on a phone on the clinic WiFi: padlock, no
warning, nothing installed on the phone.

---

## When it doesn't work

**Name doesn't resolve on phones, but the server is fine.**
Some routers refuse DNS answers containing private IPs ("DNS rebinding protection") —
common on OpenWRT, Fritz!Box, pfSense and Pi-hole. Test with
`nslookup clinic.yourdomain.com` from a phone on the clinic WiFi. If the server resolves
it but devices don't, that's the cause: disable rebinding protection on the router, or
add an exception for your domain.

**`certificate obtained` never appears.**

```sh
docker compose logs caddy | grep -i -e acme -e error
```

- *"API token … appears invalid"* → you pasted the token with quotes or braces around
  it. Paste just the token.
- *"invalid zone"* / 403 → the token lacks `Zone:DNS:Edit`, or is scoped to another zone
- *timed out waiting for record* → the domain isn't on Cloudflare's nameservers yet
- *too many certificates* → weekly limit hit. Wait, and use staging next time.

**Warning page on every device.** You're still on staging. Check `CARE_ACME_CA`.

**Everything loads but uploads or `/admin` fail.** The frontend is still built for the
old address: `care rebuild-frontend`.

**Server moved / IP changed.** Normally handled automatically — see below. If it
didn't happen, run `care dns-sync` and read the error it prints.

---

## Moving networks and changing addresses

The router hands this computer an address, and that changes: on a reboot, on a new
lease, and every time the clinic connects to a different network. A stale record fails
in the least helpful way possible — valid certificate, healthy server, and devices
simply can't find it.

So **the server maintains the record itself**, using the same Cloudflare token it
already needs for renewal:

- on every `care start`, before anything else comes up
- while the desktop app is open, within a minute of the address changing
- on demand with `care dns-sync`

It only writes when the address actually differs, and it pins `proxied: false` — a
proxied "orange cloud" record cannot reach a private address and would take the clinic
offline. If the record has been deleted, it's recreated.

### When the server is on two networks

Auto-detection asks the machine *"which of your addresses reaches the internet?"* That
is the same as *"which address do clients use"* only when clients are on that same
network. If the server is on **two** networks at once, the two differ:

```
internet  ←── 192.168.1.50    ← auto-detected (the uplink)
                [ SERVER ]
phones    ←── 192.168.4.1     ← what clients actually connect to
```

Publishing the uplink address sends every device to something that doesn't exist on
their network. So when the server is on two networks, pin the client-facing one:

```sh
CARE_LAN_IP=192.168.4.1
```

**The rule: set `CARE_LAN_IP` when the machine running CARE is on two networks.** In
practice that means it is itself the WiFi hotspot — a Raspberry Pi running as the
access point is the usual case, where `192.168.4.1` is the Pi's own AP address.

Behind an ordinary router the server has a single address, clients are on that same
network, and auto-detection is already correct — leave it empty.

Note this is separate from the local DNS entry, which you need either way. The DNS
entry is how *clients* resolve the name offline; `CARE_LAN_IP` is which of the
*server's own* addresses gets published. Different problems, different boxes.

**What the sync does and doesn't cover.** Reconnecting to a different WiFi works, as
long as **that network has internet** — the server needs it to update the record, and, more
fundamentally, clinic devices need it to look the name up at all. On a network with no
internet, a phone cannot resolve `clinic.yourdomain.com` even though the server is
metres away. That is inherent to using a public domain, not something the sync can fix.

If you need a clinic that works with no internet at all — a mobile van, say — a public
domain is the wrong tool, and the design has to change rather than be patched.

---

## Who can reach it

Only devices on the same network. This is structural, not a rule that could be
misconfigured: the domain resolves to a **private** address like `192.168.1.50`, and
private addresses aren't routable across the internet. Someone on mobile data who
types your domain gets that address back, their phone tries it on *their* network, and
reaches nothing. There is no port forwarding and no public address anywhere.

Two things worth knowing: the **name** is public (anyone can look it up, and
Certificate Transparency logs publish it), and on another network that happens to use
the same private range a device may reach some unrelated machine and fail confusingly.
Neither exposes the clinic.

---

## How it works

```
Nurse's phone ──── clinic WiFi ────► server :443   (never leaves the building)
                                        │
Cloudflare DNS ◄────────────────────────┘  once every ~60 days, to renew
```

- The server can't use the usual certificate check, which needs Let's Encrypt to
  connect *in* — it sits behind the router's NAT with no port forwarding, deliberately.
  Instead it uses **DNS-01**: it writes a TXT record through the Cloudflare API, which
  Let's Encrypt reads from anywhere. Only outbound internet is required.
- Only port **443** is published. There is no port 80 and no HTTP redirect.
- Certificates live in the `caddy-data` Docker volume. `care uninstall` removes it, and
  reinstalling re-issues (counting against the weekly limit). Avoid
  `docker compose down -v` for the same reason.
- `tls.env` holds a live credential. It is gitignored, written `0600`, and preserved
  across app updates. `tls.env.example` is the tracked template.

---

## What's hardened, and why

Serving HTTPS is not the whole job — the settings around it have to match, or the
encryption can be worked around.

| Setting | Where | Why |
|---|---|---|
| `SESSION_COOKIE_SECURE`, `CSRF_COOKIE_SECURE` | `clinic_settings.py` | The browser refuses to send login cookies over plain http at all. Without this, a single stray `http://` request to the clinic's host would put a valid session cookie on the wire in the clear. |
| `Strict-Transport-Security` | `Caddyfile` | After one successful visit, the browser refuses plain http for this name entirely. Set at the edge so it covers the React app and the file buckets, not just Django's own responses. |
| `SECURE_PROXY_SSL_HEADER` | `clinic_settings.py` | Tells Django the browser's original scheme. Without it every request looks insecure and Django builds `http://` absolute URLs, breaking presigned file links and `/admin` redirects. |
| `CORS_ALLOW_ALL_ORIGINS = False` | `clinic_settings.py` | Everything is one origin behind Caddy, so nothing legitimate is cross-origin. The old allow-all let any website on the internet read unauthenticated API responses from a clinic device's browser. |
| `X-Content-Type-Options: nosniff` | `Caddyfile` + Django | Stops a browser from second-guessing a response's declared type — the usual route to turning an uploaded file into executable script. |
| `-Server` | `Caddyfile` | Caddy names itself in every response by default. No reason to tell the network what's running. |

**Why HSTS still matters even with no port 80.** The server publishes only 443, so
there is nothing of ours to strip. But an attacker on the clinic WiFi can still answer
a stray `http://clinic.yourdomain.com` request themselves — from their own machine — and
serve a convincing fake login page. HSTS means the browser never makes that request in
the first place. It only protects after one successful HTTPS visit, which is inherent
to the mechanism.

`CARE_HSTS_SECONDS` defaults to 30 days. Staff devices visit daily, so they stay
continuously protected, while a mistake ages out in a month rather than a year.
`preload` is deliberately **not** set: it is submitted to browser vendors and is
extremely hard to undo.
