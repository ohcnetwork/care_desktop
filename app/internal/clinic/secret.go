package clinic

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
)

// genSecret replaces DJANGO_SECRET_KEY=CHANGE_ME in backend.env with a random
// key. crypto/rand - strong, and no python/shell needed.
func (e *Clinic) genSecret() error {
	path := filepath.Join(e.InstallDir, "backend.env")
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(b)
	if !strings.Contains(text, "DJANGO_SECRET_KEY=CHANGE_ME") {
		return nil
	}
	raw := make([]byte, 40)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	key := base64.RawURLEncoding.EncodeToString(raw) // ~54 url-safe chars
	text = strings.Replace(text, "DJANGO_SECRET_KEY=CHANGE_ME", "DJANGO_SECRET_KEY="+key, 1)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return err
	}
	e.logln("Generated a random DJANGO_SECRET_KEY in backend.env")
	return nil
}
