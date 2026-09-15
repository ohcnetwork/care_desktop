package trust

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestLinuxRemovalChecksEveryAnchor(t *testing.T) {
	for _, tc := range []struct {
		name         string
		firstPresent bool
		approve      bool
		removeLast   bool
		wantCalls    []bool
		wantRemoved  bool
		wantError    bool
	}{
		{"Fedora only", false, true, true, []bool{false, true}, true, false},
		{"partial unprivileged removal", true, true, true, []bool{false, true}, true, false},
		{"elevation failed silently", false, true, false, []bool{false, true}, false, true},
		{"approval declined", false, false, false, []bool{false}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			anchors := []string{filepath.Join(dir, "debian root.crt"), filepath.Join(dir, "fedora root.crt")}
			if tc.firstPresent {
				if err := os.WriteFile(anchors[0], []byte("public root"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(anchors[1], []byte("public root"), 0o644); err != nil {
				t.Fatal(err)
			}
			var calls []bool
			removed, err := removeLinuxAnchors(anchors, nil, func(string, string) bool { return tc.approve },
				func(_ string, elevated bool) error {
					calls = append(calls, elevated)
					if !elevated {
						if err := os.Remove(anchors[0]); err != nil && !os.IsNotExist(err) {
							t.Fatal(err)
						}
						return errors.New("permission denied")
					}
					if tc.removeLast {
						if err := os.Remove(anchors[1]); err != nil {
							t.Fatal(err)
						}
					}
					return nil
				})
			if removed != tc.wantRemoved || (err != nil) != tc.wantError || !slices.Equal(calls, tc.wantCalls) {
				t.Fatalf("removed = %v, err = %v, calls = %v", removed, err, calls)
			}
		})
	}
}

func TestLinuxRemovalSkipsCommandsWhenAllAnchorsAreAbsent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "missing.crt")
	removed, err := removeLinuxAnchors([]string{p}, []string{p + ".bundle"}, nil, func(string, bool) error {
		t.Fatal("attempted removal for absent anchors")
		return nil
	})
	if removed || err != nil {
		t.Fatalf("removed = %v, err = %v", removed, err)
	}
}

func TestLinuxRemovalRetriesWhenBundleStillTrustsRoot(t *testing.T) {
	dir := t.TempDir()
	anchor := filepath.Join(dir, "care-root.crt")
	bundle := filepath.Join(dir, "ca-bundle.crt")
	if err := os.WriteFile(bundle, append(certPEM(t, "unrelated root"), certPEM(t, CommonName)...), 0o644); err != nil {
		t.Fatal(err)
	}
	present, err := linuxTrustPresent([]string{anchor}, []string{bundle})
	if err != nil || !present {
		t.Fatalf("present = %v, err = %v", present, err)
	}
	var calls []bool
	removed, err := removeLinuxAnchors([]string{anchor}, []string{bundle}, func(string, string) bool { return true },
		func(sh string, elevated bool) error {
			calls = append(calls, elevated)
			for _, want := range []string{"set -e", "rm -f", "command -v update-ca-certificates", "command -v update-ca-trust", `[ "$updated" -eq 1 ]`} {
				if !strings.Contains(sh, want) {
					t.Fatalf("removal script is missing %q:\n%s", want, sh)
				}
			}
			if !elevated {
				return errors.New("permission denied")
			}
			return os.WriteFile(bundle, certPEM(t, "unrelated root"), 0o644)
		})
	if !removed || err != nil || !slices.Equal(calls, []bool{false, true}) {
		t.Fatalf("removed = %v, err = %v, calls = %v", removed, err, calls)
	}
	present, err = linuxTrustPresent([]string{anchor}, []string{bundle})
	if err != nil || present {
		t.Fatalf("present = %v, err = %v", present, err)
	}
}

func TestLinuxInspectionReportsUnreadableBundle(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read every file")
	}
	dir := t.TempDir()
	bundle := filepath.Join(dir, "ca-bundle.crt")
	if err := os.WriteFile(bundle, certPEM(t, CommonName), 0o000); err != nil {
		t.Fatal(err)
	}
	present, err := linuxTrustPresent([]string{filepath.Join(dir, "missing.crt")}, []string{bundle})
	if err == nil || present {
		t.Fatalf("present = %v, err = %v", present, err)
	}
	removed, err := removeLinuxAnchors([]string{filepath.Join(dir, "missing.crt")}, []string{bundle}, nil, func(string, bool) error {
		t.Fatal("attempted removal without a readable inspection")
		return nil
	})
	if removed || err == nil {
		t.Fatalf("removed = %v, err = %v", removed, err)
	}
}

func certPEM(t *testing.T, commonName string) []byte {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: commonName},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, public, private)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestTrustBundleIsReadAgainAfterChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ca-bundle.crt")
	empty := x509.NewCertPool()
	if !rootsFromFiles([]string{path}).Equal(empty) {
		t.Fatal("missing bundle was not empty")
	}
	data := certPEM(t, "fixture root")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	expected := x509.NewCertPool()
	expected.AppendCertsFromPEM(data)
	if !rootsFromFiles([]string{path}).Equal(expected) {
		t.Fatal("newly installed root was not loaded")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if !rootsFromFiles([]string{path}).Equal(empty) {
		t.Fatal("removed root was cached")
	}
}
