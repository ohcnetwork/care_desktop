package care

import "testing"

// mdnsLabel must reduce any user spelling to a bare DNS label.
func TestMDNSLabel(t *testing.T) {
	cases := map[string]string{
		"care":         "care",
		"care.local":   "care",
		" care.local ": "care",
		"care.local.":  "care",
		"clinic":       "clinic",
		"":             "",
	}
	for in, want := range cases {
		if got := mdnsLabel(in); got != want {
			t.Errorf("mdnsLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

// Resolves must never panic and must report false for a nil/stopped responder -
// the safe path the watchdog relies on to re-advertise instead of crashing.
func TestResolvesSafeOnStopped(t *testing.T) {
	var nilAdv *Advertiser
	if nilAdv.Resolves() {
		t.Error("nil Advertiser.Resolves() = true, want false")
	}
	stopped := &Advertiser{name: "care"} // server == nil
	if stopped.Resolves() {
		t.Error("stopped Advertiser.Resolves() = true, want false")
	}
}

// may legitimately return an error - that's not a test failure.
func TestLANIPv4s(t *testing.T) {
	ips, err := lanIPv4s()
	if err != nil {
		t.Skipf("no LAN IPv4 available here: %v", err)
	}
	for _, ip := range ips {
		if ip.To4() == nil {
			t.Errorf("non-IPv4 leaked: %v", ip)
		}
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			t.Errorf("unwanted address leaked: %v", ip)
		}
	}
}
