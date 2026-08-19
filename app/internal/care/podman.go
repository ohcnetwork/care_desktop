package care

import (
	"encoding/json"
	"fmt"
	"strconv"
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
	Fixable    bool   `json:"fixable"` // FixPodmanMachineMemory can raise the memory in one step
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

// PodmanMachineCheck reports whether podman's VM is started and has enough
// memory to build CARE's images without crashing mid-build. Only meaningful
// once containerBin() has resolved to podman; a machine-less setup (Docker, or
// native Linux podman) reports Applicable: false.
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
			Applicable: true, OK: false,
			Message: "podman machine '" + m.Name + "' is stopped",
			How:     "Start it: podman machine start" + machineArg(m.Name),
		}
	}
	mem := m.memoryMiB()
	if mem > 0 && mem < MinPodmanMachineMemoryMiB {
		return PodmanMachineStatus{
			Applicable: true, OK: false, Fixable: true,
			Message: fmt.Sprintf("podman machine '%s' has only %dMiB of memory", m.Name, mem),
			How: fmt.Sprintf("Builds (especially the frontend's Vite build) need at least %dMiB or the machine's VM can crash mid-build. "+
				"Click Fix, or by hand: podman machine stop%s && podman machine set --memory %d%s && podman machine start%s",
				MinPodmanMachineMemoryMiB, machineArg(m.Name), MinPodmanMachineMemoryMiB, machineArg(m.Name), machineArg(m.Name)),
		}
	}
	return PodmanMachineStatus{Applicable: true, OK: true, Message: fmt.Sprintf("podman machine '%s' running, %dMiB memory", m.Name, mem)}
}

// FixPodmanMachineMemory raises the default podman machine's memory to
// MinPodmanMachineMemoryMiB. podman requires the machine to be stopped to
// resize it, so this stops it (dropping any running containers - the caller
// should warn first), resizes, then starts it back up.
func (e *Engine) FixPodmanMachineMemory() error {
	machines, ok := e.podmanMachines()
	if !ok {
		return fmt.Errorf("no podman machine found")
	}
	m := defaultPodmanMachine(machines)
	e.logln("Stopping podman machine '" + m.Name + "' to resize it...")
	stopArgs := []string{"machine", "stop"}
	if m.Name != "" {
		stopArgs = append(stopArgs, m.Name)
	}
	if err := e.run(nil, "podman", stopArgs...); err != nil {
		return fmt.Errorf("couldn't stop podman machine '%s': %w", m.Name, err)
	}
	e.logln(fmt.Sprintf("Setting podman machine '%s' memory to %dMiB...", m.Name, MinPodmanMachineMemoryMiB))
	setArgs := []string{"machine", "set", "--memory", strconv.Itoa(MinPodmanMachineMemoryMiB)}
	if m.Name != "" {
		setArgs = append(setArgs, m.Name)
	}
	if err := e.run(nil, "podman", setArgs...); err != nil {
		return fmt.Errorf("couldn't resize podman machine '%s': %w", m.Name, err)
	}
	e.logln("Starting podman machine '" + m.Name + "'...")
	startArgs := []string{"machine", "start"}
	if m.Name != "" {
		startArgs = append(startArgs, m.Name)
	}
	if err := e.run(nil, "podman", startArgs...); err != nil {
		return fmt.Errorf("couldn't start podman machine '%s': %w", m.Name, err)
	}
	return nil
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
