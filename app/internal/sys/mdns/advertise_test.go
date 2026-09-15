package mdns

import (
	"errors"
	"net"
	"testing"
	"time"

	hmdns "github.com/hashicorp/mdns"
)

func serviceEntry() *hmdns.ServiceEntry {
	return &hmdns.ServiceEntry{
		Name: "care._https._tcp.local.", Host: "care.local.", Port: 443,
		InfoFields: []string{"CARE Desktop"}, AddrV4: net.ParseIP("192.0.2.10"),
	}
}

func TestAdvertiserMatchesMulticastIdentity(t *testing.T) {
	a := &Advertiser{name: "care", ips: []net.IP{net.ParseIP("192.0.2.10"), net.ParseIP("198.51.100.10")}}
	if !a.matches(serviceEntry()) || a.matches(nil) {
		t.Fatal("incorrect base identity matching")
	}
	for _, tc := range []struct {
		name   string
		change func(*hmdns.ServiceEntry)
	}{
		{"another instance", func(e *hmdns.ServiceEntry) { e.Name = "other._https._tcp.local." }},
		{"another service", func(e *hmdns.ServiceEntry) { e.Name = "care._http._tcp.local." }},
		{"another host", func(e *hmdns.ServiceEntry) { e.Host = "other.local." }},
		{"another port", func(e *hmdns.ServiceEntry) { e.Port = 80 }},
		{"another advertiser", func(e *hmdns.ServiceEntry) { e.InfoFields = []string{"Other service"} }},
		{"hosts loopback answer", func(e *hmdns.ServiceEntry) { e.AddrV4 = net.ParseIP("127.0.0.1") }},
		{"stale address", func(e *hmdns.ServiceEntry) { e.AddrV4 = net.ParseIP("192.0.2.11") }},
		{"missing address", func(e *hmdns.ServiceEntry) { e.AddrV4 = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entry := serviceEntry()
			tc.change(entry)
			if a.matches(entry) {
				t.Fatalf("unrelated response matched: %+v", entry)
			}
		})
	}
	entry := serviceEntry()
	entry.Name, entry.Host = "CARE._HTTPS._TCP.LOCAL.", "CARE.LOCAL."
	entry.AddrV4 = net.ParseIP("198.51.100.10")
	if !a.matches(entry) {
		t.Fatal("another current LAN address or DNS case was rejected")
	}
}

func TestResolvesUsesBoundedMDNSQuery(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entry   *hmdns.ServiceEntry
		err     error
		stopped bool
		want    bool
	}{
		{"matching multicast response", serviceEntry(), nil, false, true},
		{"no multicast response", nil, nil, false, false},
		{"query failure", serviceEntry(), errors.New("multicast unavailable"), false, false},
		{"stopped during query", serviceEntry(), nil, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &Advertiser{name: "care", ips: []net.IP{net.ParseIP("192.0.2.10")}, server: &hmdns.Server{}}
			got := a.resolves(func(params *hmdns.QueryParam) error {
				if params.Service != "_https._tcp" || params.Domain != "local" ||
					params.Timeout != 2*time.Second || params.WantUnicastResponse {
					t.Errorf("unexpected multicast query: %+v", params)
				}
				if tc.entry != nil {
					params.Entries <- tc.entry
				}
				if tc.stopped {
					a.mu.Lock()
					a.server = nil
					a.mu.Unlock()
				}
				return tc.err
			})
			if got != tc.want {
				t.Fatalf("Resolves = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStoppedAdvertiserDoesNotQuery(t *testing.T) {
	for _, a := range []*Advertiser{nil, {name: "care"}} {
		if a.resolves(func(*hmdns.QueryParam) error {
			t.Error("queried for a stopped advertiser")
			return nil
		}) {
			t.Fatal("stopped advertiser resolved")
		}
	}
}

func TestSameIPsRetainsAddressChangeDetection(t *testing.T) {
	a := []net.IP{net.ParseIP("192.0.2.10"), net.ParseIP("198.51.100.10")}
	if !sameIPs(a, []net.IP{a[1], a[0]}) || sameIPs(a, a[:1]) ||
		sameIPs(a, []net.IP{a[0], net.ParseIP("198.51.100.11")}) {
		t.Fatal("LAN address set comparison failed")
	}
}
