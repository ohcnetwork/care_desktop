package clinic

func (e *Clinic) Stop() error    { e.logln("Stopping CARE (data kept)..."); return e.dc("stop") }
func (e *Clinic) Restart() error { e.logln("Restarting CARE..."); return e.dc("restart") }
