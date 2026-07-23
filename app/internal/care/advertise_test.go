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

// lanIPv4s must only ever return non-loopback IPv4s. On a box with just loopback it
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
