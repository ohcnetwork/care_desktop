package clinic

// RebuildBackend rebuilds the backend image (after new backend code) and restarts
// the Django + celery services, then migrates.
func (e *Clinic) RebuildBackend() error {
	if err := e.Builder().BuildBackend(); err != nil {
		return err
	}
	// Recreate + migrate the api backend BEFORE celery-beat (which also migrates),
	// so the two don't race on the freshly-built code's new migrations.
	if err := e.dc("up", "-d", "backend"); err != nil {
		return err
	}
	if err := e.migrate(); err != nil {
		return err
	}
	if err := e.dc("up", "-d", "celery-worker", "celery-beat"); err != nil {
		return err
	}
	e.logln("Backend rebuilt and restarted.")
	return nil
}

// RebuildFrontend rebuilds the FE image (Vite bakes REACT_* at build time) and
// restarts the frontend service.
func (e *Clinic) RebuildFrontend() error {
	if err := e.Builder().BuildFrontend(); err != nil {
		return err
	}
	if err := e.dc("up", "-d", "frontend"); err != nil {
		return err
	}
	e.logln("Frontend rebuilt and restarted.")
	return nil
}
