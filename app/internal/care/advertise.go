package care

import (
	"fmt"
	"net"
	"strings"

	"github.com/hashicorp/mdns"
)

// Advertiser answers mDNS queries for "<name>.local" with this host's LAN IPv4
// address(es). It runs as a HOST process - never inside a container, since Docker
// Desktop's VM can't multicast onto the physical LAN - so the same pure-Go path
// works identically on macOS, Linux, and Windows. Unlike renaming the machine's
// hostname it needs no sudo and changes nothing on the box; it only resolves while
// running (which is fine: the app serves nothing when it's down anyway).
type Advertiser struct {
	name   string
	ips    []net.IP
	server *mdns.Server
}

// Advertise starts answering "<name>.local?" on the LAN. name is the host label
// with or without a trailing ".local" (e.g. "care" or "care.local"). Returns a
// running Advertiser; call Stop to shut it down.
func Advertise(name string) (*Advertiser, error) {
	name = mdnsLabel(name)
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
	if a == nil || a.server == nil {
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

// newMDNSServer builds the responder. The hostName ("<name>.local.") is the record
// a browser's "care.local" A-query matches (verified in hashicorp/mdns zone.go).
func newMDNSServer(name string, ips []net.IP) (*mdns.Server, error) {
	svc, err := mdns.NewMDNSService(
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
	return mdns.NewServer(&mdns.Config{Zone: svc})
}

// mdnsLabel normalises a user-supplied name to a bare DNS label: trims spaces, then
// any surrounding dots (so a trailing "." doesn't defeat the suffix strip), then a
// trailing ".local".
func mdnsLabel(name string) string {
	name = strings.Trim(strings.TrimSpace(name), ".")
	name = strings.TrimSuffix(name, ".local")
	return strings.Trim(name, ".")
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
