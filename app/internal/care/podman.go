package care

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// MinPodmanMachineMemoryMiB is the minimum we recommend for the podman machine
// VM (macOS/Windows only - native Linux podman has no VM). The care_fe
// Dockerfile alone requests a 4GiB Node heap (NODE_OPTIONS=--max-old-space-size=4096)
// for its Vite build; a VM with less memory than that reliably OOMs *the VM
// itself* mid-build (not just the build step), which surfaces as a cryptic
// "server probably quit: unexpected EOF" rather than a normal build error.
const MinPodmanMachineMemoryMiB = 4096

// PodmanMachineStatus reports whether podman's VM (used on macOS/Windows; native
// Linux podman has none) is running and adequately sized. Applicable is false
// when there's no machine to check - not using podman, or a machine-less
// (native Linux) podman - and the wizard hides the row in that case.
type PodmanMachineStatus struct {
	Applicable bool   `json:"applicable"`
	OK         bool   `json:"ok"`
	Message    string `json:"message"`
	How        string `json:"how"`
	Fixable    bool   `json:"fixable"`  // one of the Fix* methods below can repair this in one step
	Blocking   bool   `json:"blocking"` // true: the stack is guaranteed to fail as-is (stopped/rootless); false: a risk, not a certainty (low memory - only builds are at risk)
}

// podmanMachine mirrors the fields we need from `podman machine list --format json`.
type podmanMachine struct {
	Name    string `json:"Name"`
	Running bool   `json:"Running"`
	Memory  string `json:"Memory"` // bytes, as a decimal string
}

func (m podmanMachine) memoryMiB() int {
	b, err := strconv.ParseInt(m.Memory, 10, 64)
	if err != nil || b <= 0 {
		return 0
	}
	return int(b / (1024 * 1024))
}

// podmanMachines lists podman's machines, if any. Returns ok=false (no error)
// when podman has no machine concept here (native Linux) or none exist yet -
// callers treat that as "nothing to check", not a failure.
func (e *Engine) podmanMachines() (machines []podmanMachine, ok bool) {
	out, err := e.capture("podman", "machine", "list", "--format", "json")
	if err != nil {
		return nil, false
	}
	if err := json.Unmarshal([]byte(out), &machines); err != nil || len(machines) == 0 {
		return nil, false
	}
	return machines, true
}

// defaultPodmanMachine returns the running machine if there is one, else the
// first (machines are typically a single default entry).
func defaultPodmanMachine(machines []podmanMachine) podmanMachine {
	for _, m := range machines {
		if m.Running {
			return m
		}
	}
	return machines[0]
}

// podmanMachineRootful reports whether the named machine (empty = the default)
// runs rootful. ok=false means it couldn't be determined (treated as "don't
// know" by callers, not as rootless).
func (e *Engine) podmanMachineRootful(name string) (rootful bool, ok bool) {
	args := []string{"machine", "inspect", "--format", "{{.Rootful}}"}
	if name != "" {
		args = append(args, name)
	}
	out, err := e.capture("podman", args...)
	if err != nil {
		return false, false
	}
	switch strings.TrimSpace(out) {
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		return false, false
	}
}

// PodmanMachineCheck reports whether podman's VM is started, can bind the
// privileged ports (80/443) CARE's Caddy needs, and has enough memory to build
// CARE's images without crashing mid-build. Only meaningful once containerBin()
// has resolved to podman; a machine-less setup (Docker, or native Linux podman)
// reports Applicable: false. Checked in the order a fresh install actually hits
// them: stopped (nothing works) -> rootless (compose up fails outright on
// `bind: permission denied` for :80/:443) -> underpowered (builds can crash).
func (e *Engine) PodmanMachineCheck() PodmanMachineStatus {
	if e.containerBin() != "podman" {
		return PodmanMachineStatus{Applicable: false, OK: true, Message: "not using Podman"}
	}
	machines, ok := e.podmanMachines()
	if !ok {
		// No machine (e.g. native Linux podman, or `podman machine init` never run).
		return PodmanMachineStatus{Applicable: false, OK: true, Message: "no podman machine on this OS"}
	}
	m := defaultPodmanMachine(machines)
	if !m.Running {
		return PodmanMachineStatus{
			Applicable: true, OK: false, Blocking: true,
			Message: "podman machine '" + m.Name + "' is stopped",
			How:     "Start it: podman machine start" + machineArg(m.Name),
		}
	}
	if rootful, known := e.podmanMachineRootful(m.Name); known && !rootful {
		return PodmanMachineStatus{
			Applicable: true, OK: false, Fixable: true, Blocking: true,
			Message: "podman machine '" + m.Name + "' is running rootless",
			How: "CARE's Caddy needs to bind ports 80/443, which rootless podman can't do " +
				"(fails with 'cannot expose privileged port 80... permission denied'). " +
				"Click Fix, or by hand: podman machine stop" + machineArg(m.Name) +
				" && podman machine set --rootful" + machineArg(m.Name) +
				" && podman machine start" + machineArg(m.Name),
		}
	}
	mem := m.memoryMiB()
	if mem > 0 && mem < MinPodmanMachineMemoryMiB {
		return PodmanMachineStatus{
			Applicable: true, OK: false, Fixable: true, // Blocking: false - a risk, not certain: the stack may still come up fine
			Message: fmt.Sprintf("podman machine '%s' has only %dMiB of memory", m.Name, mem),
			How: fmt.Sprintf("Builds (especially the frontend's Vite build) need at least %dMiB or the machine's VM can crash mid-build. "+
				"Click Fix, or by hand: podman machine stop%s && podman machine set --memory %d%s && podman machine start%s",
				MinPodmanMachineMemoryMiB, machineArg(m.Name), MinPodmanMachineMemoryMiB, machineArg(m.Name), machineArg(m.Name)),
		}
	}
	return PodmanMachineStatus{Applicable: true, OK: true, Message: fmt.Sprintf("podman machine '%s' running, %dMiB memory", m.Name, mem)}
}

// podmanMachineStopSetStart stops the default machine, runs `machine set
// <setArgs...>`, then starts it back up - the stop/reconfigure/start dance
// every podman machine setting change (memory, rootful, ...) needs.
func (e *Engine) podmanMachineStopSetStart(action string, setArgs ...string) error {
	machines, ok := e.podmanMachines()
	if !ok {
		return fmt.Errorf("no podman machine found")
	}
	m := defaultPodmanMachine(machines)
	e.logln("Stopping podman machine '" + m.Name + "' to " + action + "...")
	if err := e.podmanMachineCmd("stop", m.Name); err != nil {
		return fmt.Errorf("couldn't stop podman machine '%s': %w", m.Name, err)
	}
	e.logln(fmt.Sprintf("%s podman machine '%s'...", strings.ToUpper(action[:1])+action[1:], m.Name))
	if err := e.podmanMachineCmd("set", m.Name, setArgs...); err != nil {
		return fmt.Errorf("couldn't reconfigure podman machine '%s': %w", m.Name, err)
	}
	e.logln("Starting podman machine '" + m.Name + "'...")
	if err := e.podmanMachineCmd("start", m.Name); err != nil {
		return fmt.Errorf("couldn't start podman machine '%s': %w", m.Name, err)
	}
	return nil
}

// podmanMachineCmd runs `podman machine <verb> [setArgs...] [name]` - name goes
// last, matching how `podman machine set --memory N <name>` etc. are invoked.
func (e *Engine) podmanMachineCmd(verb, name string, args ...string) error {
	full := append([]string{"machine", verb}, args...)
	if name != "" {
		full = append(full, name)
	}
	return e.run(nil, "podman", full...)
}

// FixPodmanMachineMemory raises the default podman machine's memory to
// MinPodmanMachineMemoryMiB. podman requires the machine to be stopped to
// resize it, so this stops it (dropping any running containers - the caller
// should warn first), resizes, then starts it back up.
func (e *Engine) FixPodmanMachineMemory() error {
	return e.podmanMachineStopSetStart("resize", "--memory", strconv.Itoa(MinPodmanMachineMemoryMiB))
}

// FixPodmanMachineRootful switches the default podman machine to rootful mode,
// so it can bind the privileged ports (80/443) CARE's Caddy needs. podman
// requires the machine stopped to reconfigure it, so this stops it (dropping
// any running containers - the caller should warn first), switches mode, then
// starts it back up.
func (e *Engine) FixPodmanMachineRootful() error {
	return e.podmanMachineStopSetStart("switch to rootful mode on", "--rootful")
}

// machineArg renders " <name>" for commands that take an optional machine name
// (podman defaults to its one machine when omitted, but naming it is clearer
// in messages users copy-paste).
func machineArg(name string) string {
	if name == "" {
		return ""
	}
	return " " + name
}
