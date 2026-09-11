// Package release holds CARE's release pins: which container images and which
// source refs an install runs.
//
// They live in one file, .env, and have no defaults anywhere - not in Go, not in
// docker-compose.yml. Every pin must be declared or the app refuses to start. A
// second place for a version to live is how BACKEND_IMAGE once drifted, with Go
// running a locally built image while a hand-run `docker compose` pulled an
// unpinned one off the internet.
//
// Operator choices (clinic address, admin password, backup folder) are NOT here.
// They are fields on clinic.Clinic, passed in for the run that needs them, so a
// password can never arrive from the environment or from .env - which Compose
// interpolates into every service.
//
// See docs/configuration.md.
package release

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/compose-spec/compose-go/v2/dotenv"
)

// EnvFile is the name Compose auto-loads from a project directory. Reading the
// same file the same way is the point: the app and a hand-run `docker compose`
// resolve identical images.
const EnvFile = ".env"

// Pins is a validated set of release pins. Load guarantees every field is set, so
// callers can use them directly without checking.
type Pins struct {
	PostgresImage string
	RedisImage    string
	MinioImage    string
	CaddyImage    string // upstream base; the caddy service runs CaddyWafImage
	CorazaVersion string // WAF module ref compiled into CaddyWafImage

	BackupImage   string
	CaddyWafImage string // CaddyImage with the Coraza WAF compiled in
	BackendImage  string
	FrontendImage string

	BeRepo string
	FeRepo string
	BeRef  string
	FeRef  string
}

// fields maps each .env key to the field it fills. It is the only place a pin is
// listed: Load both validates and populates from it, so adding a pin cannot leave
// half the work done.
func (p *Pins) fields() []struct {
	key string
	dst *string
} {
	return []struct {
		key string
		dst *string
	}{
		{"POSTGRES_IMAGE", &p.PostgresImage},
		{"REDIS_IMAGE", &p.RedisImage},
		{"MINIO_IMAGE", &p.MinioImage},
		{"CADDY_IMAGE", &p.CaddyImage},
		{"CORAZA_VERSION", &p.CorazaVersion},
		{"BACKUP_IMAGE", &p.BackupImage},
		{"CADDY_WAF_IMAGE", &p.CaddyWafImage},
		{"BACKEND_IMAGE", &p.BackendImage},
		{"FRONTEND_IMAGE", &p.FrontendImage},
		{"CARE_BE_REPO", &p.BeRepo},
		{"CARE_FE_REPO", &p.FeRepo},
		{"CARE_BE_REF", &p.BeRef},
		{"CARE_FE_REF", &p.FeRef},
	}
}

// Load parses .env and fails unless every pin is present, naming all the missing
// ones at once. It uses the parser Docker Compose itself uses, so quoting,
// escapes and ${VAR} expansion resolve identically on both sides - a mismatch
// there once left MinIO and the backend disagreeing about a password by two
// quote characters.
func Load(env []byte) (*Pins, error) {
	values, err := dotenv.Parse(bytes.NewReader(env))
	if err != nil {
		return nil, fmt.Errorf("%s is malformed: %w", EnvFile, err)
	}
	var p Pins
	var missing []string
	for _, f := range p.fields() {
		v := strings.TrimSpace(values[f.key])
		if v == "" {
			missing = append(missing, f.key)
			continue
		}
		*f.dst = v
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("%s is missing required values: %s", EnvFile, strings.Join(missing, ", "))
	}
	return &p, nil
}
