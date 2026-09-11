// Package mdns advertises "<name>.local" on the LAN and reports whether it
// resolves. See docs/architecture.md#mdns-advertising-and-self-heal.
package mdns

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	hmdns "github.com/hashicorp/mdns"
)

// Advertiser answers mDNS queries for "<name>.local" with this host's LAN IPv4
// address(es). It runs as a HOST process - never inside a container, since Docker
// Desktop's VM can't multicast onto the physical LAN - so the same pure-Go path
// works identically on macOS, Linux, and Windows. Unlike renaming the machine's
// hostname it needs no sudo and changes nothing on the box; it only resolves while
// running (which is fine: the app serves nothing when it's down anyway).
type Advertiser struct {
	name string
	ips  []net.IP // fixed at construction; IPsChanged compares against them

	// The watchdog probes an Advertiser from its own goroutine while the UI can
	// Stop the same one (a name change in Settings, or quitting mid-probe), so
	// server is the one field two goroutines touch.
	mu     sync.Mutex
	server *hmdns.Server
}

// Advertise starts answering "<name>.local?" on the LAN. name is the host label
// with or without a trailing ".local" (e.g. "care" or "care.local"). Returns a
// running Advertiser; call Stop to shut it down.
func Advertise(name string) (*Advertiser, error) {
	name = Label(name)
	if name == "" {
		return nil, fmt.Errorf("empty mDNS name")
	}
	ips, err := lanIPv4s()
	if err != nil {
		return nil, err
	}
	server, err := newMDNSServer(name, ips)
	if err != nil {
		return nil, err
	}
	return &Advertiser{name: name, ips: ips, server: server}, nil
}

// Name is the bare label being advertised (e.g. "care").
func (a *Advertiser) Name() string { return a.name }

// Stop shuts the responder down (idempotent, nil-safe).
func (a *Advertiser) Stop() {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.server == nil {
		return
	}
	_ = a.server.Shutdown()
	a.server = nil
}

// IPsChanged reports whether the host's current LAN IPv4s differ from what this
// Advertiser is announcing - the app polls this to catch DHCP address changes and
// re-advertise, so "care.local" doesn't point at a stale IP.
func (a *Advertiser) IPsChanged() bool {
	cur, err := lanIPv4s()
	if err != nil {
		return false // transient; keep the current advertisement
	}
	return !sameIPs(a.ips, cur)
}

// Resolves reports whether "<name>.local" currently answers on this host - the
// watchdog's liveness check, so a responder that silently stopped (macOS
// mDNSResponder dropped our record, sleep/wake, network flap) gets restarted.
// ponytail: uses the system resolver (handles mDNS on macOS/Windows; on Linux
// only with nss-mdns). Where it can't resolve .local it just reports false and
// the caller re-advertises - cheap and harmless. Debounce lives in the caller.
func (a *Advertiser) Resolves() bool {
	if a == nil || a.stopped() {
		return false
	}
	// Deliberately not holding the lock across the lookup: it can take the full
	// two seconds, and Stop must not block on a quit waiting for a DNS timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupHost(ctx, a.name+".local")
	if err != nil || len(addrs) == 0 {
		return false
	}
	return true
}

func (a *Advertiser) stopped() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.server == nil
}

// NameStatus reports whether this machine is reachable as <name>.local, with a
// per-OS "how" the wizard shows when it isn't.
type NameStatus struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	How     string `json:"how"`
}

// Check verifies that <name>.local actually resolves right now - a real
// functional test (does the LAN answer?), uniform across OSes. It's gated in the
// installer because the frontend is baked to http://care.local. The app answers
// this itself via Advertise, so status goes green as soon as its responder is up.
// name is the bare label (e.g. "care").
func Check(name string) NameStatus {
	full := name + ".local"
	if _, err := net.LookupHost(full); err == nil {
		return NameStatus{OK: true, Message: full + " resolves"}
	}
	how := "Open (and keep open) the CARE Desktop app - it advertises " + full +
		" on the LAN while running. Then re-check."
	if runtime.GOOS == "windows" {
		how += "\nOn Windows, also allow inbound UDP 5353 (PowerShell as Admin):\n" +
			"  Set-NetConnectionProfile -NetworkCategory Private\n" +
			"  New-NetFirewallRule -DisplayName \"mDNS\" -Direction Inbound -Protocol UDP -LocalPort 5353 -Action Allow -Profile Private"
	}
	how += "\nStill failing? Use a static IP (see the install docs)."
	return NameStatus{OK: false, Message: full + " isn't resolving yet", How: how}
}

// newMDNSServer builds the responder. The hostName ("<name>.local.") is the record
// a browser's "care.local" A-query matches (verified in hashicorp/mdns zone.go).
func newMDNSServer(name string, ips []net.IP) (*hmdns.Server, error) {
	svc, err := hmdns.NewMDNSService(
		name,           // instance name
		"_http._tcp",   // service type (also lists it for DNS-SD browsers)
		"local.",       // domain
		name+".local.", // hostName - answers A/AAAA for <name>.local
		80,             // port
		ips,
		[]string{"CARE Desktop"},
	)
	if err != nil {
		return nil, err
	}
	return hmdns.NewServer(&hmdns.Config{Zone: svc})
}

// Label normalises a user-supplied name to a bare DNS label: trims spaces, then
// any surrounding dots (so a trailing "." doesn't defeat the suffix strip), then a
// trailing ".local".
func Label(name string) string {
	name = strings.ToLower(strings.Trim(strings.TrimSpace(name), "."))
	name = strings.TrimSuffix(name, ".local")
	return strings.Trim(name, ".")
}

var labelRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)

// ValidateLabel checks a user-supplied name. The installer calls it as the
// user types.
func ValidateLabel(name string) error {
	label := strings.ToLower(Label(name))
	switch {
	case label == "":
		return fmt.Errorf("enter a name, for example care")
	case len(label) > 63:
		return fmt.Errorf("name is too long (63 characters at most)")
	case !labelRe.MatchString(label):
		return fmt.Errorf("use lowercase letters, numbers and hyphens only, for example care-test")
	}
	return nil
}

// lanIPv4s returns this host's usable IPv4 addresses (interfaces that are up and not
// loopback; link-local 169.254.x.x excluded).
func lanIPv4s() ([]net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var ips []net.IP
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
				ips = append(ips, ip4)
			}
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no usable LAN IPv4 address found")
	}
	return ips, nil
}

func sameIPs(a, b []net.IP) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]bool{}
	for _, ip := range a {
		seen[ip.String()] = true
	}
	for _, ip := range b {
		if !seen[ip.String()] {
			return false
		}
	}
	return true
}
