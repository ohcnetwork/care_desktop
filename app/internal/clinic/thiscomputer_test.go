package clinic

import (
	"errors"
	"strings"
	"testing"
)

func TestLocalSetupRequiresVerifiedHostsAndTrust(t *testing.T) {
	for _, tc := range []struct {
		name       string
		hostsReady bool
		trustReady bool
		err        error
		want       string
	}{
		{"verified", true, true, nil, "can now open"},
		{"verified despite command failure", true, true, errors.New("command failed"), "can now open"},
		{"missing hosts", false, true, nil, "Could not confirm the local hosts entry."},
		{"missing trust", true, false, nil, "Could not confirm certificate trust."},
		{"missing both", false, false, nil, "Could not confirm the local hosts entry and certificate trust."},
		{"failed elevation", true, false, errors.New("approval declined"), "certificate trust (approval declined)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			message := localSetupResult("care.local", tc.hostsReady, tc.trustReady, tc.err)
			if !strings.Contains(message, tc.want) {
				t.Fatalf("unexpected setup result: %q", message)
			}
			if (!tc.hostsReady || !tc.trustReady) &&
				(strings.Contains(message, "can now open") || !strings.Contains(message, "Other devices are unaffected")) {
				t.Fatalf("optional local setup failure was misreported: %q", message)
			}
			if strings.Contains(message, "/setup") {
				t.Fatalf("setup result refers to the retired web page: %q", message)
			}
			if (!tc.hostsReady || !tc.trustReady) && !strings.Contains(message, "try starting CARE again") {
				t.Fatalf("setup failure must explain how to retry: %q", message)
			}
		})
	}
}
