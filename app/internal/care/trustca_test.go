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
	"strings"
	"testing"
	"time"
)

// certSHA1Hex must return the uppercase hex SHA-1 the OS trust tools use to match a
// cert, and must be empty (never panic) on junk input - the untrust path deletes by
// this fingerprint, so a wrong value would miss the cert or target the wrong one.
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
