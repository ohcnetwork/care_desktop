package trust

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientAddress(t *testing.T) {
	for _, address := range []string{"care", "CARE.local", " https://care.local/ ", "http://care.local"} {
		got, err := ClientURL(address)
		if err != nil || got != "https://care.local" {
			t.Fatalf("%q: %q, %v", address, got, err)
		}
	}
	for _, address := range []string{
		"", "https://", "https://care.local:443", "care.local:", "care.local/setup", "https://person@care.local",
		"care.local?x=1", "care.local?", "care.local#", "127.0.0.1", "[::1]", "clinic.example.com",
		"file:///etc/passwd", "care.local/../", "care.local/%2f", "-care.local", "care..local",
		"care.local\nbad", "care;echo.local",
	} {
		if _, err := ClientURL(address); err == nil {
			t.Errorf("accepted invalid address %q", address)
		}
	}
}

func TestClientCertificateDownloadValidation(t *testing.T) {
	root, _ := clientTestTLS(t, time.Now().Add(time.Hour))
	for _, tc := range []struct {
		name, body string
		status     int
		valid      bool
	}{
		{"root", root, 200, true},
		{"redirect", root, 302, false},
		{"missing", "not found", 404, false},
		{"html", "<html>setup page</html>", 200, false},
		{"oversized", strings.Repeat("x", 64*1024+1), 200, false},
		{"bundle", root + root, 200, false},
		{"trailing content", root + "unexpected data", 200, false},
		{"wrong root", string(certPEM(t, "Other service")), 200, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}
			got, err := readClientCertificate(response)
			if (err == nil) != tc.valid || (tc.valid && got != root) {
				t.Fatalf("certificate validation = %q, %v", got, err)
			}
		})
	}
	expired, _ := clientTestTLS(t, time.Now().Add(-time.Hour))
	if _, err := clientRoot(expired, true); err == nil {
		t.Fatal("expired root was accepted for installation")
	}
	if _, err := clientRoot(expired, false); err != nil {
		t.Fatalf("expired root cannot be removed: %v", err)
	}
}

func TestClientTLSUsesClinicNameAndPinnedRoot(t *testing.T) {
	root, certificate := clientTestTLS(t, time.Now().Add(time.Hour))
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{certificate}}
	server.StartTLS()
	defer server.Close()
	for _, tc := range []struct {
		name, host, root string
		ok               bool
	}{
		{"clinic", "care.local", root, true},
		{"wrong host", "other.local", root, false},
		{"replaced root", "care.local", string(certPEM(t, CommonName)), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, err := clientTLSConfig(tc.host, tc.root)
			if err != nil {
				t.Fatal(err)
			}
			conn, err := tls.Dial("tcp", server.Listener.Addr().String(), config)
			if conn != nil {
				_ = conn.Close()
			}
			if (err == nil) != tc.ok {
				t.Fatalf("TLS verification: %v", err)
			}
		})
	}
}

func TestClientRemovalTargetsOnlyItsCertificate(t *testing.T) {
	root := string(certPEM(t, CommonName))
	cert, err := clientRoot(root, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, goos := range []string{"darwin", "windows", "linux"} {
		step, err := clientCertificateStep(goos, root, "/tmp/clinic certificate.crt", false)
		if err != nil || !strings.Contains(step.Sh+step.PS, "/tmp/clinic certificate.crt") {
			t.Fatalf("%s install: %+v, %v", goos, step, err)
		}
		step, err = clientCertificateStep(goos, root, "", true)
		if err != nil {
			t.Fatal(err)
		}
		script := step.Sh + step.PS
		want := SHA1Hex(root)
		if goos == "linux" {
			want = clientAnchors(cert)[0]
			if strings.Contains(script, "/care-root.crt") {
				t.Fatal("client removal targets server certificate")
			}
		}
		if !strings.Contains(script, want) || strings.Contains(script, CommonName) {
			t.Fatalf("%s removal must target only the exact certificate: %s", goos, script)
		}
	}
	if _, err := clientCertificateStep("windows", "bad root", "", true); err == nil {
		t.Fatal("invalid certificate accepted for removal")
	}
}

func clientTestTLS(t *testing.T, notAfter time.Time) (string, tls.Certificate) {
	t.Helper()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: CommonName},
		NotBefore: time.Now().Add(-2 * time.Hour), NotAfter: notAfter,
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	root := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2), DNSNames: []string{"care.local"},
		NotBefore: ca.NotBefore, NotAfter: ca.NotAfter,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, ca, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	return root, tls.Certificate{Certificate: [][]byte{leafDER}, PrivateKey: key}
}
