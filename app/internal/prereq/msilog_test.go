package prereq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeMSILog(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "install.log")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMSIFailureDetailReportsTheBlockingCondition(t *testing.T) {
	body := strings.Join([]string{
		`MSI (s) (1C:40) [13:33:58:123]: Product: Rancher Desktop -- Rancher Desktop requires Windows Subsystem for Linux 2 (WSL2) to be installed as a prerequisite. Please follow the instructions at https://aka.ms/wslinstall`,
		`MSI (s) (1C:40) [13:33:59:456]: Product: Rancher Desktop -- Installation failed.`,
		`MSI (s) (1C:40) [13:33:59:789]: Windows Installer installed the product. Installation success or error status: 1603.`,
	}, "\r\n")

	detail := msiFailureDetail(writeMSILog(t, body))
	if !strings.Contains(detail, "WSL2") {
		t.Fatalf("the blocking condition was not surfaced: %q", detail)
	}
	if strings.Contains(detail, "Installation failed") {
		t.Fatalf("a generic status line was reported instead of the cause: %q", detail)
	}
}

func TestMSIFailureDetailSkipsGenericStatusOnlyLogs(t *testing.T) {
	body := "MSI (s) (1C:40): Product: Rancher Desktop -- Installation failed.\r\n" +
		"MSI (s) (1C:40): Product: Rancher Desktop -- Installation success or error status: 1603.\r\n"
	if detail := msiFailureDetail(writeMSILog(t, body)); detail != "" {
		t.Fatalf("generic status lines must not be reported as a cause: %q", detail)
	}
}

func TestMSIFailureDetailReadsUTF16Logs(t *testing.T) {
	line := "MSI (s) (1C:40): Product: Rancher Desktop -- Rancher Desktop requires WSL2.\r\n"
	var utf16ish strings.Builder
	for _, r := range line {
		utf16ish.WriteRune(r)
		utf16ish.WriteByte(0)
	}
	if detail := msiFailureDetail(writeMSILog(t, utf16ish.String())); !strings.Contains(detail, "WSL2") {
		t.Fatalf("a UTF-16 log hid the cause: %q", detail)
	}
}

func TestMSIFailureDetailToleratesAMissingLog(t *testing.T) {
	if detail := msiFailureDetail(filepath.Join(t.TempDir(), "absent.log")); detail != "" {
		t.Fatalf("a missing log should report no cause, got %q", detail)
	}
}
