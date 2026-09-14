package clinic

func (e *Clinic) pruneBuildCache() {
	e.logln("Pruning Docker build cache (shared with any other projects on this computer)...")
	if err := e.run(nil, "docker", "builder", "prune", "-f"); err != nil {
		e.logln("  (couldn't prune the build cache - continuing)")
	}
}

func (e *Clinic) uninstallImages() []string {
	return []string{
		e.Pins.BackendImage, e.Pins.FrontendImage, e.Pins.CaddyWafImage, e.Pins.BackupImage,
		e.Pins.PostgresImage, e.Pins.RedisImage, e.Pins.MinioImage,
		e.Pins.CaddyImage, e.Pins.CaddyImage + "-builder",
	}
}

func (e *Clinic) removeImage(tag string) {
	if _, err := e.capture("docker", "image", "rm", tag); err != nil {
		e.logln("  skipped " + tag + " (not present or still in use)")
		return
	}
	e.logln("  removed " + tag)
}
