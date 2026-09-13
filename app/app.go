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
	if name == "" {
		return
	}
	adv, err := mdns.Advertise(name)
	if err != nil {
		a.logln("mDNS: couldn't advertise " + name + ".local (" + err.Error() + ")")
		return
	}
	a.adv = adv
}

func (a *App) restartAdvertise() {
	a.advMu.Lock()
	a.adv.Stop()
	a.adv = nil
	a.advMu.Unlock()
	a.startAdvertise()
}

func (a *App) advRunning() bool {
	a.advMu.Lock()
	defer a.advMu.Unlock()
	return a.adv != nil
}

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
				a.startAdvertise()
				continue
			}
			if adv.IPsChanged() {
				misses = 0
				a.restartAdvertise()
				continue
			}
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
