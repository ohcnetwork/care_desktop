package care

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The untrust path deletes by this fingerprint: wrong value = wrong cert, or none.
func TestCertSHA1Hex(t *testing.T) {
	for _, in := range []string{"", "not a pem", "-----BEGIN CERTIFICATE-----\nnope\n-----END CERTIFICATE-----"} {
		if got := certSHA1Hex(in); got != "" {
			t.Errorf("certSHA1Hex(%q) = %q, want empty", in, got)
		}
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test.ca"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	sum := sha1.Sum(der)
	want := strings.ToUpper(hex.EncodeToString(sum[:]))
	if got := certSHA1Hex(string(pemBytes)); got != want {
		t.Errorf("certSHA1Hex = %q, want %q", got, want)
	}
}

// A missed root stays trusted forever.
func TestParseSHA1Hashes(t *testing.T) {
	raw := `SHA-1 hash: 8CFFE4093CB688BC168776661793DD12C80E0BB3
keychain: "/Users/x/Library/Keychains/login.keychain-db"
    "alis"<blob>="CARE Desktop Local CA"
SHA-1 hash: 0b335e4018b87c4540dde5d8faca5eadc63b6093
    "alis"<blob>="CARE Desktop Local CA"
SHA-1 hash: 8CFFE4093CB688BC168776661793DD12C80E0BB3
`
	got := parseSHA1Hashes(raw)
	want := []string{
		"8CFFE4093CB688BC168776661793DD12C80E0BB3",
		"0B335E4018B87C4540DDE5D8FACA5EADC63B6093", // uppercased to match the OS tools
	}
	if len(got) != len(want) {
		t.Fatalf("got %d hashes %v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("hash %d = %q, want %q", i, got[i], want[i])
		}
	}
	if len(parseSHA1Hashes("")) != 0 {
		t.Error("empty output must yield no hashes, not a bogus entry")
	}
}

// Guards the regression where hardcoded tags stopped matching versions.env.
func TestUninstallImageListMatchesPinnedVersions(t *testing.T) {
	e := &Engine{Kit: t.TempDir()}
	if err := os.WriteFile(filepath.Join(e.Kit, "versions.env"), []byte(
		"POSTGRES_IMAGE=postgres:17.10-alpine\nREDIS_IMAGE=redis:8.8.0-alpine\nCADDY_IMAGE=caddy:2.11.4\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"postgres:17.10-alpine", "redis:8.8.0-alpine", "caddy:2.11.4"} {
		found := false
		for _, got := range e.uninstallImages() {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%q missing from uninstall image list %v", want, e.uninstallImages())
		}
	}
}
