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

type Trace struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Detail string `json:"detail"`
}

type Report struct {
	Clean  bool    `json:"clean"`
	Traces []Trace `json:"traces"`
}

type Options struct {
	Runner       proc.Runner
	Project      string
	InstallDir   string
	ConfigPath   string
	Images       []string
	StoredSecret bool
}

func Scan(o Options) Report {
	var traces []Trace
	add := func(id, label, detail string) {
		traces = append(traces, Trace{ID: id, Label: label, Detail: detail})
	}

	label := "label=com.docker.compose.project=" + o.Project

	if n := len(o.Runner.Lines("docker", "ps", "-aq", "--filter", label)); n > 0 {
		add("containers", "Clinic containers", plural(n, "container", "containers")+" from an earlier install")
	}

	if n := len(o.Runner.Lines("docker", "volume", "ls", "-q", "--filter", label)); n > 0 {
		add("volumes", "Old clinic data", plural(n, "data volume", "data volumes")+" from an earlier install")
	}
	if n := len(o.Runner.Lines("docker", "network", "ls", "-q", "--filter", label)); n > 0 {
		add("networks", "Clinic network", plural(n, "network", "networks")+" from an earlier install")
	}
	if n := len(presentImages(o.Runner, o.Images)); n > 0 {
		add("images", "Clinic images", plural(n, "Docker image", "Docker images")+" from an earlier install")
	}

	if hasComposeFile(o.InstallDir) {
		add("install-dir", "Installed files", o.InstallDir)
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

func InstallDirFrom(run proc.Runner, project, configured string) string {
	if hasComposeFile(configured) {
		return configured
	}
	for _, dir := range run.Lines("docker", "ps", "-a",
		"--filter", "label=com.docker.compose.project="+project,
		"--format", `{{index .Labels "com.docker.compose.project.working_dir"}}`) {
		if dir = strings.TrimSpace(dir); hasComposeFile(dir) {
			return dir
		}
	}
	return configured
}

func hasComposeFile(dir string) bool {
	if dir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, "docker-compose.yml"))
	return err == nil
}

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
