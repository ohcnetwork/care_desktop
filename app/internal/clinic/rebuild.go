package clinic

func (e *Clinic) RebuildBackend() error {
	if err := e.Builder().BuildBackend(); err != nil {
		return err
	}
	if err := e.dc("up", "-d", "--wait", "--wait-timeout", "300", "backend"); err != nil {
		return err
	}
	if err := e.migrate(); err != nil {
		return err
	}
	if err := e.dc("up", "-d", "--wait", "--wait-timeout", "300", "celery-worker", "celery-beat"); err != nil {
		return err
	}
	e.logln("Backend rebuilt and restarted.")
	return nil
}

func (e *Clinic) RebuildFrontend() error {
	if err := e.Builder().BuildFrontend(); err != nil {
		return err
	}
	if err := e.dc("up", "-d", "--wait", "--wait-timeout", "300", "frontend"); err != nil {
		return err
	}
	e.logln("Frontend rebuilt and restarted.")
	return nil
}
