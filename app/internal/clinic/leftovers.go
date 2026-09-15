package clinic

// reportLeftovers closes the run with an honest account. "Uninstall complete" on
// its own is a claim the operator can't check, and the two things most likely to
// survive - a hosts line or a trusted root the admin prompt was declined for -
// are exactly the ones worth naming.
func (e *Clinic) reportLeftovers(opts UninstallOptions, failed []string) {
	var kept []string
	if !opts.RemoveBackups {
		if dir := e.backupDir(); dirExists(dir) {
			kept = append(kept, "Backups in "+dir)
		}
	}
	if !opts.RemoveImages {
		kept = append(kept, "Downloaded Docker images and build cache")
	}

	e.logln("")
	if len(failed) == 0 && len(kept) == 0 {
		e.logln("Clinic resources removed.")
		return
	}
	if len(failed) == 0 {
		e.logln("Clinic resources removed. Kept on purpose:")
	} else {
		e.logln("Uninstall finished, but these could NOT be reverted:")
		for _, s := range failed {
			e.logln("  ! " + s)
		}
		if len(kept) > 0 {
			e.logln("Kept on purpose:")
		}
	}
	for _, s := range kept {
		e.logln("  - " + s)
	}
}
