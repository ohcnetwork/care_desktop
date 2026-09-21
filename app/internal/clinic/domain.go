package clinic

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/compose-spec/compose-go/v2/dotenv"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/atomicfile"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/mdns"
)

var domainFiles = []string{"Caddyfile", "backend.env", "frontend.env"}

var domainEnvKeys = map[string][]string{
	"backend.env":  {"CSRF_TRUSTED_ORIGINS", "BUCKET_EXTERNAL_ENDPOINT"},
	"frontend.env": {"REACT_CARE_API_URL"},
}

var hostTokenRe = regexp.MustCompile(`[A-Za-z0-9_.-]+`)
var caddyHostRe = regexp.MustCompile(`(?m)^([A-Za-z0-9-]+\.local):443[ \t]*\{\r?\n[ \t]*tls internal\r?\n[ \t]*import bootstrap\r?\n[ \t]*import site\r?\n\}`)
var domainAssignmentRe = regexp.MustCompile(`^[ \t]*(?:export[ \t]+)?([A-Za-z_][A-Za-z0-9_]*)[ \t]*(?:=[ \t]*|:[ \t]+)`)

func (e *Clinic) ApplyDomain() error {
	if err := mdns.ValidateLabel(e.mdnsName()); err != nil {
		return err
	}
	host := e.host()
	managedHosts := map[string]bool{"example.local": true}
	files := make(map[string][]byte, len(domainFiles))
	for _, rel := range domainFiles {
		path := filepath.Join(e.InstallDir, filepath.FromSlash(rel))
		b, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		files[rel] = b
		if keys, ok := domainEnvKeys[rel]; ok {
			env, err := dotenv.Parse(bytes.NewReader(b))
			if err != nil {
				return fmt.Errorf("%s: %w", rel, err)
			}
			for _, key := range keys {
				u, err := url.Parse(env[key])
				if err == nil && strings.HasSuffix(strings.ToLower(u.Hostname()), ".local") {
					managedHosts[strings.ToLower(u.Hostname())] = true
				}
			}
		}
	}
	for _, match := range caddyHostRe.FindAllStringSubmatch(string(files["Caddyfile"]), -1) {
		managedHosts[strings.ToLower(match[1])] = true
	}
	replace := func(text string) string {
		return hostTokenRe.ReplaceAllStringFunc(text, func(token string) string {
			if managedHosts[strings.ToLower(token)] {
				return host
			}
			return token
		})
	}
	for _, rel := range domainFiles {
		b, ok := files[rel]
		if !ok {
			continue
		}
		var out string
		if keys, ok := domainEnvKeys[rel]; ok {
			out = replaceDomainSettings(string(b), keys, replace)
		} else {
			out = replace(string(b))
		}
		if out == string(b) {
			continue
		}
		path := filepath.Join(e.InstallDir, filepath.FromSlash(rel))
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if err := atomicfile.Write(path, []byte(out), info.Mode().Perm()); err != nil {
			return err
		}
	}
	e.logln("Clinic address set to https://" + host + "/")
	return nil
}

func replaceDomainSettings(text string, keys []string, replace func(string) string) string {
	var out strings.Builder
	for len(text) > 0 {
		lineEnd := strings.IndexByte(text, '\n')
		if lineEnd < 0 {
			lineEnd = len(text)
		} else {
			lineEnd++
		}
		match := domainAssignmentRe.FindStringSubmatchIndex(text)
		if match == nil {
			out.WriteString(text[:lineEnd])
			text = text[lineEnd:]
			continue
		}
		start, end := match[1], lineEnd
		if start < len(text) && (text[start] == '\'' || text[start] == '"') {
			for i := start + 1; i < len(text); i++ {
				if text[i] == '\\' {
					i++
				} else if text[i] == text[start] {
					end = i + 1
					break
				}
			}
			if next := strings.IndexByte(text[end:], '\n'); next >= 0 {
				lineEnd = end + next + 1
			} else {
				lineEnd = len(text)
			}
		} else if comment := strings.IndexByte(text[start:end], '#'); comment >= 0 {
			end = start + comment
		}
		if slices.Contains(keys, text[match[2]:match[3]]) {
			out.WriteString(text[:start])
			out.WriteString(replace(text[start:end]))
			out.WriteString(text[end:lineEnd])
		} else {
			out.WriteString(text[:lineEnd])
		}
		text = text[lineEnd:]
	}
	return out.String()
}

func (e *Clinic) host() string {
	return mdns.Label(e.mdnsName()) + ".local"
}

func (e *Clinic) Host() string { return e.host() }
