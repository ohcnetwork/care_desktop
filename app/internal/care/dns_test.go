package care

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeCloudflare stands in for the API. It records what was sent so the tests can
// assert on the request body, which is where the damaging mistakes would be (a
// proxied record, or the wrong address).
type fakeCloudflare struct {
	zones   []map[string]string // id/name pairs the token can see
	records []map[string]string // id/content for the A record, empty = none exists
	method  string              // last write method (POST = create, PUT = update)
	sent    map[string]any      // last write body
	fail    string              // when set, every call returns this error message
}

func (f *fakeCloudflare) start(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("missing/wrong bearer token: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		if f.fail != "" {
			json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"errors":  []map[string]any{{"code": 1000, "message": f.fail}},
			})
			return
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/zones") && strings.Contains(r.URL.Path, "dns_records"):
			if r.Method == "GET" {
				json.NewEncoder(w).Encode(map[string]any{"success": true, "result": f.records})
				return
			}
			f.method = r.Method
			_ = json.NewDecoder(r.Body).Decode(&f.sent)
			json.NewEncoder(w).Encode(map[string]any{"success": true, "result": map[string]string{}})
		case r.URL.Path == "/zones":
			json.NewEncoder(w).Encode(map[string]any{"success": true, "result": f.zones})
		default:
			json.NewEncoder(w).Encode(map[string]any{"success": true, "result": map[string]string{}})
		}
	}))
	t.Cleanup(srv.Close)
	old := cloudflareAPI
	cloudflareAPI = srv.URL
	t.Cleanup(func() { cloudflareAPI = old })
}

// OutboundIP must return the address on the network this machine can actually reach,
// never a loopback and never a Docker bridge.
func TestOutboundIP(t *testing.T) {
	ip, err := OutboundIP()
	if err != nil {
		t.Skip("no usable network interface in this environment")
	}
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() == nil {
		t.Fatalf("not an IPv4 address: %q", ip)
	}
	if parsed.IsLoopback() || parsed.IsUnspecified() {
		t.Fatalf("unusable address for a DNS record: %q", ip)
	}
}

// The zone must be found by matching the token's own zones, not by chopping labels
// off the host - which gets multi-part suffixes and subdomain zones wrong.
func TestZoneForPicksLongestMatch(t *testing.T) {
	f := &fakeCloudflare{zones: []map[string]string{
		{"id": "short", "name": "example.co.uk"},
		{"id": "long", "name": "clinics.example.co.uk"},
		{"id": "other", "name": "unrelated.com"},
	}}
	f.start(t)
	got, err := newCloudflare("tok").zoneFor("van.clinics.example.co.uk")
	if err != nil {
		t.Fatal(err)
	}
	if got != "long" {
		t.Fatalf("zone: got %q, want the most specific match", got)
	}
}

func TestZoneForRejectsUncoveredDomain(t *testing.T) {
	f := &fakeCloudflare{zones: []map[string]string{{"id": "z", "name": "someone-else.com"}}}
	f.start(t)
	_, err := newCloudflare("tok").zoneFor("clinic.example.com")
	if err == nil || !strings.Contains(err.Error(), "doesn't cover") {
		t.Fatalf("expected a scoping error, got %v", err)
	}
}

// The common case: the record already holds this address, so nothing is written.
// Start calls this every time, so a needless write on every boot would be noise at
// best and rate-limit pressure at worst.
func TestSyncDNSNoOpWhenCorrect(t *testing.T) {
	ip, err := OutboundIP()
	if err != nil {
		t.Skip("no usable network interface")
	}
	f := &fakeCloudflare{
		zones:   []map[string]string{{"id": "z", "name": "example.com"}},
		records: []map[string]string{{"id": "r", "content": ip}},
	}
	f.start(t)
	if err := tlsEngine(t, goodTLS).SyncDNS(); err != nil {
		t.Fatal(err)
	}
	if f.method != "" {
		t.Fatalf("wrote to Cloudflare when the record was already correct (%s)", f.method)
	}
}

func TestSyncDNSUpdatesStaleRecord(t *testing.T) {
	ip, err := OutboundIP()
	if err != nil {
		t.Skip("no usable network interface")
	}
	f := &fakeCloudflare{
		zones:   []map[string]string{{"id": "z", "name": "example.com"}},
		records: []map[string]string{{"id": "r", "content": "10.9.9.9"}},
	}
	f.start(t)
	if err := tlsEngine(t, goodTLS).SyncDNS(); err != nil {
		t.Fatal(err)
	}
	if f.method != "PUT" {
		t.Fatalf("expected an update, got %q", f.method)
	}
	assertRecord(t, f.sent, ip)
}

func TestSyncDNSCreatesMissingRecord(t *testing.T) {
	ip, err := OutboundIP()
	if err != nil {
		t.Skip("no usable network interface")
	}
	f := &fakeCloudflare{zones: []map[string]string{{"id": "z", "name": "example.com"}}}
	f.start(t)
	if err := tlsEngine(t, goodTLS).SyncDNS(); err != nil {
		t.Fatal(err)
	}
	if f.method != "POST" {
		t.Fatalf("expected a create, got %q", f.method)
	}
	assertRecord(t, f.sent, ip)
}

// An unconfigured install must not call Cloudflare at all - the app starts this
// watcher before the wizard has run.
func TestSyncDNSNoopWhenUnconfigured(t *testing.T) {
	f := &fakeCloudflare{fail: "should never be called"}
	f.start(t)
	if err := tlsEngine(t, "CARE_PUBLIC_HOST=\nCLOUDFLARE_API_TOKEN=\n").SyncDNS(); err != nil {
		t.Fatalf("unconfigured install should be a silent no-op, got %v", err)
	}
}

// Cloudflare answers 200 with success:false, so the error message has to come from
// the body - not the status code.
func TestSyncDNSSurfacesAPIError(t *testing.T) {
	f := &fakeCloudflare{fail: "Invalid API Token"}
	f.start(t)
	err := tlsEngine(t, goodTLS).SyncDNS()
	if err == nil || !strings.Contains(err.Error(), "Invalid API Token") {
		t.Fatalf("API error not surfaced: %v", err)
	}
}

// CARE_LAN_IP exists for the case auto-detection gets wrong: a server on two networks
// at once, typically because it is the WiFi hotspot. Auto-detect returns the uplink
// address; clients are on the other side. Publishing the uplink sends every device to
// an address that doesn't exist on their network.
func TestLanIPOverrideWins(t *testing.T) {
	f := &fakeCloudflare{zones: []map[string]string{{"id": "z", "name": "example.com"}}}
	f.start(t)
	e := tlsEngine(t, goodTLS+"CARE_LAN_IP=192.168.4.1\n")

	if got, err := e.lanIP(); err != nil || got != "192.168.4.1" {
		t.Fatalf("lanIP() = %q, %v; want the pinned address", got, err)
	}
	if err := e.SyncDNS(); err != nil {
		t.Fatal(err)
	}
	assertRecord(t, f.sent, "192.168.4.1")
}

// With no override the address is auto-detected, which is correct for the ordinary
// case of a server sitting behind a router with a single address.
func TestLanIPFallsBackToAutoDetect(t *testing.T) {
	want, err := OutboundIP()
	if err != nil {
		t.Skip("no usable network interface")
	}
	got, err := tlsEngine(t, goodTLS).lanIP()
	if err != nil || got != want {
		t.Fatalf("lanIP() = %q, %v; want the auto-detected %q", got, err, want)
	}
}

// A typo here would publish an unreachable address to public DNS, so it must be
// rejected before anything is written rather than surfacing as "nobody can connect".
func TestLanIPRejectsNonAddress(t *testing.T) {
	for _, bad := range []string{"192.168.4", "clinic.example.com", "192.168.4.1/24", "not-an-ip"} {
		e := tlsEngine(t, goodTLS+"CARE_LAN_IP="+bad+"\n")
		if _, err := e.lanIP(); err == nil {
			t.Fatalf("accepted %q as an address", bad)
		}
		if err := e.SyncDNS(); err == nil {
			t.Fatalf("synced DNS with %q as the address", bad)
		}
	}
}

// A proxied ("orange cloud") record cannot reach a private address and would take the
// clinic offline, so every write must pin proxied:false explicitly.
func assertRecord(t *testing.T, sent map[string]any, wantIP string) {
	t.Helper()
	if sent["content"] != wantIP {
		t.Fatalf("wrote address %v, want %v", sent["content"], wantIP)
	}
	if sent["type"] != "A" {
		t.Fatalf("record type %v, want A", sent["type"])
	}
	if sent["proxied"] != false {
		t.Fatalf("proxied is %v - an orange-cloud record can't reach a private address", sent["proxied"])
	}
	if sent["ttl"] != float64(dnsTTL) {
		t.Fatalf("ttl %v, want %d so devices pick up changes quickly", sent["ttl"], dnsTTL)
	}
}
