// Package settings resolves CARE's configuration: explicit overrides, then the
// process environment, then versions.env in the install dir, then the built-in
// default. See docs/configuration.md.
package settings

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Settings reads configuration for one install directory.
type Settings struct {
	Dir string            // install dir, holding versions.env
	Env map[string]string // highest-precedence overrides

	versions map[string]string
	once     sync.Once
}

// get resolves a setting: explicit Env override > process env > versions.env > default.
func (s *Settings) get(key, def string) string {
	if s.Env != nil {
		if v, ok := s.Env[key]; ok && v != "" {
			return v
		}
	}
	if v := os.Getenv(key); v != "" {
		return v
	}
	s.once.Do(s.loadVersions)
	if v, ok := s.versions[key]; ok && v != "" {
		return v
	}
	return def
}

func (s *Settings) loadVersions() {
	s.versions = map[string]string{}
	b, err := os.ReadFile(filepath.Join(s.Dir, "versions.env"))
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			s.versions[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
}

func (s *Settings) BackendImage() string  { return s.get("BACKEND_IMAGE", "care:clinic") }
func (s *Settings) FrontendImage() string { return s.get("FRONTEND_IMAGE", "care_fe:clinic") }
func (s *Settings) PostgresImage() string { return s.get("POSTGRES_IMAGE", "postgres:17.10-alpine") }
func (s *Settings) RedisImage() string    { return s.get("REDIS_IMAGE", "redis:8.8.0-alpine") }
func (s *Settings) MinioImage() string {
	return s.get("MINIO_IMAGE", "minio/minio:RELEASE.2025-09-07T16-13-09Z")
}
func (s *Settings) CaddyImage() string  { return s.get("CADDY_IMAGE", "caddy:2.11.4") }
func (s *Settings) BackupImage() string { return s.get("BACKUP_IMAGE", "care-backup:clinic") }

// WafCaddyImage is the custom Caddy image (Coraza WAF compiled in) the caddy service
// actually runs; CaddyImage() above is the pinned base it's built from.
func (s *Settings) WafCaddyImage() string { return s.get("CADDY_WAF_IMAGE", "care-caddy:clinic") }

// BackupPassword is the passphrase that protects the backup keypair's private key.
// Empty means backup encryption is disabled - dumps are written in plaintext, and
// setup skips keypair generation. Set via CARE_BACKUP_PASSWORD at setup time.
func (s *Settings) BackupPassword() string { return s.get("CARE_BACKUP_PASSWORD", "") }
func (s *Settings) BeRepo() string {
	return s.get("CARE_BE_REPO", "https://github.com/ohcnetwork/care.git")
}
func (s *Settings) FeRepo() string {
	return s.get("CARE_FE_REPO", "https://github.com/ohcnetwork/care_fe.git")
}
func (s *Settings) BeRef() string { return s.get("CARE_BE_REF", "develop") }
func (s *Settings) FeRef() string { return s.get("CARE_FE_REF", "develop") }
func (s *Settings) BeDir() string { return s.get("CARE_BE_DIR", filepath.Join(s.Dir, "care")) }
func (s *Settings) FeDir() string { return s.get("CARE_FE_DIR", filepath.Join(s.Dir, "care_fe")) }

// MDNSName is the bare host label to advertise/resolve (e.g. "care").
func (s *Settings) MDNSName() string      { return s.get("CARE_MDNS_NAME", "care") }
func (s *Settings) AdminPassword() string { return s.get("CARE_ADMIN_PASSWORD", "admin") }
func (s *Settings) NoMDNS() bool          { return s.get("CARE_NO_MDNS", "0") == "1" }

// MDNSMode selects how http://<name>.local is made resolvable:
//   - "advertise" (default): a pure-Go mDNS responder in the app / `care mdns` -
//     no rename, no sudo, works on all 3 OSes.
//   - "rename": the old scutil/hostnamectl path (a permanent OS-level hostname).
//   - "off": do nothing (static-IP users). CARE_NO_MDNS=1 forces this too.
func (s *Settings) MDNSMode() string {
	if s.NoMDNS() {
		return "off"
	}
	switch strings.ToLower(s.get("CARE_MDNS_MODE", "advertise")) {
	case "rename":
		return "rename"
	case "off":
		return "off"
	default:
		return "advertise"
	}
}

func (s *Settings) BackupDir() string {
	if d := s.get("BACKUP_DIR", ""); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Desktop", "care-db-backups")
}
