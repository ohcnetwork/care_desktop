package clinic

import (
	"os"
	"strings"
)

const caddyRootPath = "/data/caddy/pki/authorities/local/root.crt"

func (e *Clinic) caddyRootPEM() string {
	if out, err := e.capture("docker", "compose", "exec", "-T", "caddy",
		"cat", caddyRootPath); err == nil && strings.Contains(out, "BEGIN CERTIFICATE") {
		return out
	}
	f, err := os.CreateTemp("", "care-root-*.crt")
	if err != nil {
		return ""
	}
	tmp := f.Name()
	f.Close()
	defer os.Remove(tmp)
	if _, err := e.capture("docker", "compose", "cp", "caddy:"+caddyRootPath, tmp); err != nil {
		return ""
	}
	b, err := os.ReadFile(tmp)
	if err != nil || !strings.Contains(string(b), "BEGIN CERTIFICATE") {
		return ""
	}
	return string(b)
}
