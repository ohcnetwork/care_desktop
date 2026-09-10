package clinic

// Status returns `docker compose ps` as "<service> <state>" lines.
func (e *Clinic) Status() (string, error) {
	return e.capture("docker", "compose", "ps", "--format", "{{.Service}} {{.State}}")
}
