// Package residue finds what an earlier CARE Desktop left on this computer.
//
// It exists because a fresh install onto a machine that still carries pieces of
// an old one fails in ways that are hard to read: a leftover data volume gets
// re-attached and the new install comes up holding the old clinic's patients, a
// stale hosts entry points the browser at nothing, an old root certificate makes
// the new one look untrusted. Detecting that up front, as one more prerequisite,
// turns those into a single red row with a button.
//
// Detection never elevates and never changes anything. See docs/architecture.md.
package residue

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/autostart"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/hosts"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/netfix"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/trust"
)

// Trace is one thing an earlier install left behind.
type Trace struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Detail string `json:"detail"`
}

// Report is the whole scan. Clean is the only thing the wizard gates on; Traces
// is what it lists, so the operator can see what is about to be removed.
type Report struct {
	Clean  bool    `json:"clean"`
	Traces []Trace `json:"traces"`
}

// Options is what the scan needs to know about this machine. Every field is
// read-only.
type Options struct {
	Runner       proc.Runner
	Project      string   // compose project label, e.g. care-desktop
	InstallDir   string   // where an install would have unpacked itself
	ConfigPath   string   // the app's own config.json
	CloneDirs    []string // the care / care_fe checkouts
	Images       []string // every image tag an install builds or pulls
	StoredSecret bool     // a backup password is in the OS secret store
}

// Scan looks for every trace an install can leave. Backups are deliberately not
// scanned: they are the recovery data, they are useless to a fresh install
// rather than harmful to it, and a "clean up" step that deleted them would be
// indefensible. Purge leaves them alone for the same reason.
func Scan(o Options) Report {
	var traces []Trace
	add := func(id, label, detail string) {
		traces = append(traces, Trace{ID: id, Label: label, Detail: detail})
	}

	label := "label=com.docker.compose.project=" + o.Project

	if n := len(o.Runner.Lines("docker", "ps", "-aq", "--filter", label)); n > 0 {
		add("containers", "Clinic containers", plural(n, "container", "containers")+" from an earlier install")
	}
	// The one that silently corrupts a fresh install: compose re-attaches a volume
	// whose name matches, so the "new" clinic comes up holding the old data.
	if n := len(o.Runner.Lines("docker", "volume", "ls", "-q", "--filter", label)); n > 0 {
		add("volumes", "Old clinic data", plural(n, "data volume", "data volumes")+" from an earlier install")
	}
	if n := len(o.Runner.Lines("docker", "network", "ls", "-q", "--filter", label)); n > 0 {
		add("networks", "Clinic network", plural(n, "network", "networks")+" from an earlier install")
	}
	if n := len(presentImages(o.Runner, o.Images)); n > 0 {
		add("images", "Clinic images", plural(n, "Docker image", "Docker images")+" from an earlier install")
	}

	if _, err := os.Stat(filepath.Join(o.InstallDir, "docker-compose.yml")); err == nil {
		add("install-dir", "Installed files", o.InstallDir)
	}
	for _, dir := range o.CloneDirs {
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			add("clones", "Downloaded source code", dir)
		}
	}
	if o.ConfigPath != "" && proc.FileExists(o.ConfigPath) {
		add("config", "Saved settings", o.ConfigPath)
	}

	if hosts.Present() {
		add("hosts", "Hosts file entry", "this computer still resolves the old clinic address to itself")
	}
	if trust.Present() {
		add("certificate", "Security certificate", "this computer still trusts the old clinic's certificate")
	}
	if netfix.RulesPresent(o.Runner) {
		add("firewall", "Firewall rules", "the clinic's inbound rules are still in place")
	}
	if autostart.Enabled() {
		add("autostart", "Start at login", "CARE Desktop is set to open when this computer starts")
	}
	if o.StoredSecret {
		add("secret", "Saved backup password", "the old backup password is still in this computer's password store")
	}

	return Report{Clean: len(traces) == 0, Traces: traces}
}

// presentImages returns the subset of tags that exist locally. One `docker
// images` call rather than an inspect per tag: an install has nine tags, and on
// a cold daemon nine round trips is a visible stall in the wizard.
func presentImages(run proc.Runner, tags []string) []string {
	have := map[string]bool{}
	for _, line := range run.Lines("docker", "images", "--format", "{{.Repository}}:{{.Tag}}") {
		have[strings.TrimSpace(line)] = true
	}
	var found []string
	for _, t := range tags {
		if have[t] {
			found = append(found, t)
		}
	}
	return found
}

func plural(n int, one, many string) string {
	word := many
	if n == 1 {
		word = one
	}
	return strconv.Itoa(n) + " " + word
}
