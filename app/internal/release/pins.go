package release

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/compose-spec/compose-go/v2/dotenv"
)

const EnvFile = ".env"

type Pins struct {
	PostgresImage string
	RedisImage    string
	MinioImage    string
	CaddyImage    string
	CorazaVersion string

	BackupImage   string
	CaddyWafImage string
	BackendImage  string
	FrontendImage string

	BeRepo string
	FeRepo string
	BeRef  string
	FeRef  string
}

func (p *Pins) envPinMap() []struct {
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

// Summary renders the pins as log lines.
func (p *Pins) Summary() []string {
	if p == nil {
		return nil
	}
	return []string{
		"pins: backend  " + p.BackendImage + " <- " + p.BeRepo + "@" + p.BeRef,
		"      frontend " + p.FrontendImage + " <- " + p.FeRepo + "@" + p.FeRef,
		"      base     " + strings.Join([]string{p.PostgresImage, p.RedisImage, p.MinioImage}, " · "),
		"      proxy    " + p.CaddyImage + " + coraza " + p.CorazaVersion,
	}
}

func Load(env []byte) (*Pins, error) {
	values, err := dotenv.Parse(bytes.NewReader(env))
	if err != nil {
		return nil, fmt.Errorf("%s is malformed: %w", EnvFile, err)
	}
	var p Pins
	var missing []string
	for _, f := range p.envPinMap() {
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
