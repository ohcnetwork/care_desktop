package compose

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ohcnetwork/care_desktop/app/internal/release"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

const LockFile = "channel.lock"

const (
	Backend  = "backend"
	Frontend = "frontend"
)

type Channel struct {
	Ref      string `json:"ref,omitempty"`
	Current  string `json:"current,omitempty"`
	Next     string `json:"next,omitempty"`
	Declined string `json:"declined,omitempty"`
}

type Lock struct {
	Backend  Channel `json:"backend"`
	Frontend Channel `json:"frontend"`
}

func (l Lock) Get(service string) Channel {
	if service == Frontend {
		return l.Frontend
	}
	return l.Backend
}

func (l *Lock) Set(service string, c Channel) {
	if service == Frontend {
		l.Frontend = c
		return
	}
	l.Backend = c
}

func ReadLock(dir string) Lock {
	var l Lock
	data, err := os.ReadFile(filepath.Join(dir, LockFile))
	if err != nil {
		return l
	}
	if err := json.Unmarshal(data, &l); err != nil {
		return Lock{}
	}
	return l
}

func WriteLock(dir string, l Lock) error {
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, LockFile), append(data, '\n'), 0o644)
}

func (b *Builder) updateLock(service string, fn func(*Channel)) error {
	l := ReadLock(b.dir)
	c := l.Get(service)
	fn(&c)
	l.Set(service, c)
	return WriteLock(b.dir, l)
}

const lsRemoteTimeout = 25 * time.Second

func head(run proc.Runner, repo, branch string) string {
	out, err := run.CaptureIn(lsRemoteTimeout,
		[]string{"GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "GCM_INTERACTIVE=never"},
		"git", "-c", "credential.helper=", "-c", "credential.interactive=never",
		"ls-remote", "--", repo, "refs/heads/"+branch)
	if err != nil {
		return ""
	}
	sha, _, _ := strings.Cut(out, "\t")
	sha = strings.TrimSpace(sha)
	if !release.IsCommitRef(sha) {
		return ""
	}
	return sha
}
