package clinic

import "github.com/ohcnetwork/care_desktop/app/internal/sys/proc"

func (e *Clinic) pruneBuildCache() {
	e.logln("Pruning Docker build cache (shared with any other projects on this computer)...")
	if err := e.run(nil, "docker", "builder", "prune", "-f"); err != nil {
		e.logln("  (couldn't prune the build cache - continuing)")
	}
}

// Tags come from the accessors that built them; hardcoding them here is what made
// `--images` match nothing once .env pinned real versions.
func (e *Clinic) uninstallImages() []string {
	return []string{
		e.Pins.BackendImage, e.Pins.FrontendImage, e.Pins.CaddyWafImage, e.Pins.BackupImage,
		e.Pins.PostgresImage, e.Pins.RedisImage, e.Pins.MinioImage,
		e.Pins.CaddyImage, e.Pins.CaddyImage + "-builder", // xcaddy build stage
	}
}

// removeImage deletes one image quietly, ignoring "not found" / "still in use".
func (e *Clinic) removeImage(tag string) {
	cmd := proc.Command("docker", "image", "rm", tag)
	cmd.Env = e.baseEnv()
	cmd.Dir = e.workdir()
	if cmd.Run() != nil {
		e.logln("  skipped " + tag + " (not present or still in use)")
		return
	}
	e.logln("  removed " + tag)
}
