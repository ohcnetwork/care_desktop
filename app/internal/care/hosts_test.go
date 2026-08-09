package care

import "testing"

func TestHostsHasEntry(t *testing.T) {
	const host = "care.local"
	yes := []string{
		"127.0.0.1 care.local # care-desktop",
		"192.168.1.5   care.local",
		"127.0.0.1\tcare.local\t# note",
	}
	for _, d := range yes {
		if !hostsHasEntry(d, host) {
			t.Fatalf("expected match in %q", d)
		}
	}
	no := []string{
		"",
		"# 127.0.0.1 care.local",      // fully commented out
		"127.0.0.1 other.local",       // different host
		"127.0.0.1 notcare.local # x", // must not substring-match
	}
	for _, d := range no {
		if hostsHasEntry(d, host) {
			t.Fatalf("unexpected match in %q", d)
		}
	}
}

func TestPSSingleQuote(t *testing.T) {
	if got := psSingleQuote("a'b"); got != "'a''b'" {
		t.Fatalf("got %q", got)
	}
	if got := psSingleQuote("plain"); got != "'plain'" {
		t.Fatalf("got %q", got)
	}
}
