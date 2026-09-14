package clinic

func (e *Clinic) Status() (string, error) {
	return e.capture("docker", "compose", "ps", "--format", "{{.Service}} {{.State}}")
}
