package care

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// HTTPS configuration and preflight. These checks run before the stack is touched:
// each of these mistakes otherwise surfaces minutes later as an opaque ACME timeout.
// See docs/tls.md.

// hostRe is a permissive FQDN check: labels of letters/digits/hyphens, at least one
// dot. Deliberately loose - it's here to catch pasted URLs and typos, not to be a
// standards-complete validator.
var hostRe = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?)+$`)

// ValidateTLS checks the HTTPS settings are present and coherent. Unlike an optional
// feature's validator this refuses an empty host: there is no plain-HTTP mode to fall
// back to, so an unconfigured install cannot serve anything.
func (e *Engine) ValidateTLS() error {
	return validateTLSSettings(e.publicHost(), e.dnsToken())
}

func validateTLSSettings(host, token string) error {
	host = strings.TrimSpace(host)
	switch {
	case host == "":
		return fmt.Errorf("no clinic web address set. CARE is served over HTTPS on a domain " +
			"you own - set CARE_PUBLIC_HOST in tls.env (e.g. clinic.example.com). See docs/tls.md")
	case strings.Contains(host, "://"), strings.Contains(host, "/"):
		trimmed := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://"), "/")
		return fmt.Errorf("the clinic web address must be a bare hostname, not a URL - "+
			"use %q, not %q", trimmed, host)
	case strings.HasSuffix(host, ".local"):
		return fmt.Errorf("the clinic web address is %q, but no certificate authority will ever "+
			"issue for a .local name - it's a reserved suffix. Use a real domain you own, "+
			"e.g. clinic.example.com", host)
	case !hostRe.MatchString(host):
		return fmt.Errorf("%q doesn't look like a domain name "+
			"(expected something like clinic.example.com)", host)
	case strings.TrimSpace(token) == "":
		return fmt.Errorf("no Cloudflare API token set. Caddy needs it to prove you own %q "+
			"(DNS-01) - this computer is behind the router's NAT, so Let's Encrypt can't "+
			"verify it any other way. See docs/tls.md", host)
	case strings.ContainsAny(token, `"'{} `):
		// Caddy's Cloudflare module rejects these too, but only once the stack is
		// already coming up - and pasting the token with its surrounding quotes is
		// an easy mistake to make.
		return fmt.Errorf("the Cloudflare API token contains quotes, braces, or spaces - " +
			"paste just the token itself, with nothing around it")
	}
	return nil
}

// SaveTLSSettings writes the clinic's address and token into tls.env, preserving the
// file's comments. The wizard and the Advanced panel both land here, so there's one
// place that decides what a valid setting looks like.
func (e *Engine) SaveTLSSettings(host, token string) error {
	host = strings.TrimSpace(host)
	token = strings.TrimSpace(token)
	if err := validateTLSSettings(host, token); err != nil {
		return err
	}
	path := filepath.Join(e.Kit, "tls.env")
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := setEnvValue(string(b), "CARE_PUBLIC_HOST", host)
	text = setEnvValue(text, "CLOUDFLARE_API_TOKEN", token)
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		return err
	}
	// WriteFile's mode only applies when it creates the file, and the kit ships this
	// one world-readable. It now holds a live credential, so tighten it explicitly.
	return os.Chmod(path, 0o600)
}

// CheckAddress is the installer's pre-flight on the clinic's web address. It answers
// the three questions that actually decide whether issuance will succeed, in the
// order they fail: is the setting shaped right, does Cloudflare accept the token, and
// does the name already point at this computer.
//
// The last one is the most common misconfiguration by a wide margin - a certificate
// will still issue if the A record points elsewhere, but no device in the clinic will
// reach CARE, and nothing about that failure looks DNS-shaped.
func (e *Engine) CheckAddress(host, token string) NameStatus {
	host = strings.TrimSpace(host)
	if err := validateTLSSettings(host, token); err != nil {
		return NameStatus{Message: "Not set up yet", How: err.Error()}
	}
	if err := newCloudflare(token).verify(); err != nil {
		return NameStatus{
			Message: "Cloudflare didn't accept that token",
			How: err.Error() + "\nCreate one at Cloudflare -> My Profile -> API Tokens, using the " +
				"\"Edit zone DNS\" template scoped to " + zoneOf(host) + ". See docs/tls.md.",
		}
	}
	// The address devices actually use, which is what the record must hold - not just
	// any address this machine happens to have.
	want, err := e.lanIP()
	if err != nil {
		return NameStatus{Message: "Can't tell this computer's network address", How: err.Error()}
	}
	resolved, _ := addressPointsHere(host)
	switch {
	case len(resolved) == 0:
		return NameStatus{
			Message: host + " doesn't resolve yet",
			How: "Add an A record for " + host + " in Cloudflare pointing at " + want +
				", with the proxy OFF (grey cloud). DNS changes can take a few minutes. " +
				"Once CARE is installed it keeps this record up to date by itself.",
		}
	case !contains(resolved, want):
		how := "It currently resolves to " + strings.Join(resolved, ", ") + ", but devices on the " +
			"clinic network reach this computer at " + want + ". Update the A record to match."
		// The most confusing version of this: the record points at a real address of
		// this machine, just not the one clients are on. Name it, because "it's my own
		// IP, why is it wrong?" is otherwise a long afternoon.
		if heldLocally(resolved[0]) {
			how = "It resolves to " + resolved[0] + ", which is this computer's address on a " +
				"different network. Clients reach it at " + want + " instead. " +
				"If this computer is also the WiFi hotspot, set CARE_LAN_IP to the address " +
				"clients connect to. See docs/tls.md."
		}
		return NameStatus{Message: host + " points at the wrong address", How: how}
	}
	return NameStatus{OK: true, Message: host + " points at this computer (" + want + ")"}
}

// addressPointsHere resolves host and reports the addresses it found.
func addressPointsHere(host string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return net.DefaultResolver.LookupHost(ctx, host)
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// localIPv4s returns this computer's usable IPv4 addresses (interfaces that are up
// and not loopback; link-local 169.254.x.x excluded). Used to tell the clinic which
// address their DNS record should point at.
func localIPv4s() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []string
	for _, ifi := range ifaces {
		if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifi.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip4 := ip.To4(); ip4 != nil && !ip4.IsLoopback() && !ip4.IsLinkLocalUnicast() {
				out = append(out, ip4.String())
			}
		}
	}
	return out
}

// zoneOf is the registrable domain of host, for pointing the user at the right zone
// in Cloudflare's UI. Naive last-two-labels; good enough for guidance text.
func zoneOf(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return host
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// tlsBanner is the one-line summary logged at start, so the log says what the stack
// came up as without the reader having to infer it from later messages.
func (e *Engine) tlsBanner() string {
	s := "Serving HTTPS as " + e.PublicOrigin()
	if e.acmeCA() != acmeProduction {
		s += "  [ACME: staging - browsers will NOT trust this certificate]"
	}
	return s
}
