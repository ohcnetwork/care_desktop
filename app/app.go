package main

import (
	"context"
	"fmt"
	"io/fs"
	"sync"
	"time"

	"github.com/ohcnetwork/care_desktop/app/internal/release"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/applog"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/mdns"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx       context.Context
	installFS fs.FS
	pins      *release.Pins
	log       *applog.Logger

	advMu   sync.Mutex
	adv     *mdns.Advertiser
	advStop chan struct{}
}

func NewApp(installFS fs.FS, log *applog.Logger) (*App, error) {
	proc.FixPath()
	env, err := fs.ReadFile(installFS, "install/"+release.EnvFile)
	if err != nil {
		return nil, fmt.Errorf("this build is missing its embedded %s: %w", release.EnvFile, err)
	}
	pins, err := release.Load(env)
	if err != nil {
		return nil, err
	}
	return &App{installFS: installFS, pins: pins, log: log}, nil
}

// logln is the single sink for everything the app streams - every line of docker
// and git output, every failed action. It writes to both destinations because
// they answer different questions: the event drives the live UI (and the setup
// progress bar, which regex-matches these lines), while the file is what is left
// to read afterwards, since the UI keeps only 300 lines and discards them on exit.
//
// Nil-ctx safe, because bindings can be called before Wails has started the
// runtime (and from tests); the file sink is nil-safe for the same reason.
func (a *App) logln(msg string) {
	a.log.Write(msg)
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "care-log", msg)
	}
}

func (a *App) startAdvertise() {
	a.advMu.Lock()
	defer a.advMu.Unlock()
	if a.adv != nil {
		return
	}
	name := a.loadConfig().MDNSName
	adv, err := mdns.Advertise(name)
	if err != nil {
		a.logln("mDNS: couldn't advertise " + name + ".local (" + err.Error() + ")")
		return
	}
	a.adv = adv
}

// restartAdvertise re-advertises with the current config (after the name changes,
// or the LAN IP changes).
func (a *App) restartAdvertise() {
	a.advMu.Lock()
	a.adv.Stop()
	a.adv = nil
	a.advMu.Unlock()
	a.startAdvertise()
}

// advRunning reports whether we're actively answering <name>.local.
func (a *App) advRunning() bool {
	a.advMu.Lock()
	defer a.advMu.Unlock()
	return a.adv != nil
}

// watchAdvertise re-advertises when the host's LAN IP changes (e.g. DHCP renew)
// or when care.local stops resolving (responder silently died - sleep/wake,
// network flap, mDNSResponder dropped our record). Cheap: a lookup every 30s.
func (a *App) watchAdvertise() {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	misses := 0
	for {
		select {
		case <-a.advStop:
			return
		case <-t.C:
			a.advMu.Lock()
			adv := a.adv
			a.advMu.Unlock()
			if adv == nil {
				// Advertising never got off the ground - on a clinic machine that
				// autostarts, almost always because the app was up before WiFi
				// associated, so lanIPv4s() had no address to announce. Retrying is
				// the whole point of a watchdog; skipping here left the name down
				// until someone restarted the app by hand.
				a.startAdvertise()
				continue
			}
			if adv.IPsChanged() {
				misses = 0
				a.restartAdvertise()
				continue
			}
			// ponytail: debounce two misses so one flaky lookup doesn't churn
			// the responder; a genuinely dead responder never recovers on its own.
			if adv.Resolves() {
				misses = 0
				continue
			}
			misses++
			if misses >= 2 {
				misses = 0
				a.logln("mDNS: " + adv.Name() + ".local stopped resolving - re-advertising.")
				a.restartAdvertise()
			}
		}
	}
}
