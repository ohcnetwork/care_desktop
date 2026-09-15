package mdns

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	hmdns "github.com/hashicorp/mdns"
)

type Advertiser struct {
	name   string
	ips    []net.IP
	mu     sync.Mutex
	server *hmdns.Server
}

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

func (a *Advertiser) Name() string { return a.name }

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

func (a *Advertiser) IPsChanged() bool {
	cur, err := lanIPv4s()
	if err != nil {
		return false
	}
	return !sameIPs(a.ips, cur)
}

func (a *Advertiser) Resolves() bool {
	return a.resolves(hmdns.Query)
}

func (a *Advertiser) resolves(query func(*hmdns.QueryParam) error) bool {
	if a == nil || a.stopped() {
		return false
	}
	entries := make(chan *hmdns.ServiceEntry, 32)
	params := hmdns.DefaultParams("_https._tcp")
	params.Timeout = 2 * time.Second
	params.DisableIPv6 = true
	params.Entries = entries
	result := make(chan error, 1)
	go func() {
		result <- query(params)
		close(entries)
	}()
	found := false
	for entry := range entries {
		if a.matches(entry) {
			found = true
		}
	}
	return <-result == nil && found && !a.stopped()
}

func (a *Advertiser) matches(entry *hmdns.ServiceEntry) bool {
	if entry == nil || !strings.EqualFold(entry.Name, a.name+"._https._tcp.local.") ||
		!strings.EqualFold(entry.Host, a.name+".local.") || entry.Port != 443 ||
		len(entry.InfoFields) != 1 || entry.InfoFields[0] != "CARE Desktop" {
		return false
	}
	for _, ip := range a.ips {
		if ip.Equal(entry.AddrV4) {
			return true
		}
	}
	return false
}

func (a *Advertiser) stopped() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.server == nil
}

type NameStatus struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func newMDNSServer(name string, ips []net.IP) (*hmdns.Server, error) {
	svc, err := hmdns.NewMDNSService(
		name,
		"_https._tcp",
		"local.",
		name+".local.",
		443,
		ips,
		[]string{"CARE Desktop"},
	)
	if err != nil {
		return nil, err
	}
	return hmdns.NewServer(&hmdns.Config{Zone: svc})
}

func Label(name string) string {
	name = strings.ToLower(strings.Trim(strings.TrimSpace(name), "."))
	name = strings.TrimSuffix(name, ".local")
	return strings.Trim(name, ".")
}

var labelRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)

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

var virtualIface = []string{"docker", "br-", "veth", "virbr", "vboxnet", "vmnet", "utun", "tun", "tap"}

func lanIPv4s() ([]net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var ips []net.IP
	for _, ifi := range ifaces {
		if ifi.Flags&net.FlagUp == 0 ||
			ifi.Flags&net.FlagLoopback != 0 ||
			ifi.Flags&net.FlagPointToPoint != 0 ||
			isVirtual(ifi.Name) {
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

func isVirtual(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range virtualIface {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
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
