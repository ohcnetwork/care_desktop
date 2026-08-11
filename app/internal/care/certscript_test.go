package care

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testRootPEM(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: caCommonName},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

// A shipped installer that won't parse is worse than none: the user runs it,
// sees an error, and has no idea the certificate wasn't installed.
func TestUnixInstallerIsValidShell(t *testing.T) {
	root := testRootPEM(t)
	script := unixInstaller(root, certSHA256Colons(root), "care.local")

	path := filepath.Join(t.TempDir(), "install-cert.sh")
	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("sh", "-n", path).CombinedOutput()
	if err != nil {
		t.Fatalf("generated script is not valid sh: %v\n%s", err, out)
	}
}

func TestInstallersEmbedTheCert(t *testing.T) {
	root := testRootPEM(t)
	fp := certSHA256Colons(root)
	body := strings.TrimSpace(root)

	for name, script := range map[string]string{
		"unix":    unixInstaller(root, fp, "care.local"),
		"windows": windowsInstaller(root, fp, "care.local"),
	} {
		if !strings.Contains(script, body) {
			t.Errorf("%s: certificate not embedded - the installer would have nothing to install", name)
		}
		if !strings.Contains(script, fp) {
			t.Errorf("%s: fingerprint missing - user can't check it against the setup page", name)
		}
		if !strings.Contains(script, "care.local") {
			t.Errorf("%s: clinic host not substituted", name)
		}
	}
}

// The PEM goes in via a heredoc / here-string; a delimiter collision would
// truncate the certificate silently.
func TestInstallerDelimitersCannotCollide(t *testing.T) {
	root := testRootPEM(t)
	if strings.Contains(root, "CARE_ROOT_PEM") {
		t.Fatal("PEM contains the heredoc delimiter")
	}
	if strings.Contains(root, "'@") {
		t.Fatal("PEM would close the PowerShell here-string")
	}
}

func TestCertSHA256Colons(t *testing.T) {
	if got := certSHA256Colons("not a pem"); got != "" {
		t.Fatalf("junk input should yield no fingerprint, got %q", got)
	}
	fp := certSHA256Colons(testRootPEM(t))
	if len(fp) != 95 || strings.Count(fp, ":") != 31 { // 32 bytes, colon separated
		t.Fatalf("unexpected fingerprint shape: %q", fp)
	}
}
