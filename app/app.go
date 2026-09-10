package main

import (
	"context"
	"io/fs"
	"sync"
	"time"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/mdns"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails bridge: every exported method is callable from the web UI as
// window.go.main.App.<Method>. It owns config persistence and drives the engine.
type App struct {
	ctx       context.Context
	installFS fs.FS // embedded deployment install dir

	advMu   sync.Mutex       // guards adv
	adv     *mdns.Advertiser // the running mDNS responder (advertise mode), if any
	advStop chan struct{}    // closed on shutdown to end the DHCP watcher
}

func NewApp(installFS fs.FS) *App {
	proc.FixPath() // make docker/git findable when launched from Finder/Explorer
	return &App{installFS: installFS}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// Advertise care.local right away (host process; no rename, no sudo). Doing it
	// before setup means the installer's step-3 check goes green immediately, and
	// leaving it up while the app runs keeps the clinic reachable by name.
	a.advStop = make(chan struct{})
	a.startAdvertise()
	go a.watchAdvertise()
}

// shutdown stops the responder when the app quits (wired via Wails OnShutdown).
func (a *App) shutdown(context.Context) {
	if a.advStop != nil {
		close(a.advStop)
	}
	a.advMu.Lock()
	a.adv.Stop()
	a.adv = nil
	a.advMu.Unlock()
}

// startAdvertise brings up the mDNS responder for the configured name, unless mDNS
// is in "rename"/"off" mode. Best-effort: a failure is logged, never fatal.
func (a *App) startAdvertise() {
	if a.engine(nil).MDNSMode() != "advertise" {
		return
	}
	a.advMu.Lock()
	defer a.advMu.Unlock()
	if a.adv != nil {
		return
	}
	name := a.loadConfig().MDNSName
	adv, err := mdns.Advertise(name)
	if err != nil {
		if a.ctx != nil {
			wruntime.EventsEmit(a.ctx, "care-log", "mDNS: couldn't advertise "+name+".local ("+err.Error()+")")
		}
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
				if a.ctx != nil {
					wruntime.EventsEmit(a.ctx, "care-log", "mDNS: care.local stopped resolving - re-advertising.")
				}
				a.restartAdvertise()
			}
		}
	}
}
