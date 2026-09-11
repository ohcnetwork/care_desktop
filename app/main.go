package main

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/applog"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/elevate"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

// version is stamped at release time (-ldflags "-X main.version=..."). It is the
// first line of every log file, and the first thing anyone reading one needs.
var version = "dev"

// appLog is the process-wide log file. It is a package var because fatal() can be
// reached before there is an App and must still record why. Nil-safe throughout.
var appLog *applog.Logger

//go:embed all:frontend/dist
var assets embed.FS

//go:embed all:install
var installFS embed.FS

func main() {
	// Opened before anything else can fail, so a build that cannot start still
	// leaves a reason behind on disk.
	cfg := loadConfig()
	appLog = applog.Open(cfg.LogDir)
	defer appLog.Close()
	appLog.OnFatal = func(msg string) { fatal(errors.New(msg)) }
	appLog.Header(version, cfg.InstallDir, cfg.MDNSName)

	app, err := NewApp(installFS, appLog)
	if err != nil {
		fatal(err)
	}
	// After NewApp, because the pins do not exist until it has read and validated
	// the embedded .env - which is also the failure fatal() above reports.
	for _, line := range app.pins.Summary() {
		appLog.Write(line)
	}

	err = wails.Run(&options.App{
		Title:            "CARE Desktop",
		Width:            1180,
		Height:           900,
		MinWidth:         720,
		MinHeight:        560,
		BackgroundColour: &options.RGBA{R: 249, G: 250, B: 251, A: 255},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// Wails' own diagnostics - asset server, bindings, IPC - otherwise go to
		// the println builtin and vanish in a packaged build. LogLevelProduction
		// defaults to ERROR and is undocumented; without raising it, almost
		// nothing Wails knows would reach the file.
		Logger:             appLog,
		LogLevel:           logger.INFO,
		LogLevelProduction: logger.INFO,
		HideWindowOnClose:  runtime.GOOS == "darwin",
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               singleInstanceID,
			OnSecondInstanceLaunch: app.onSecondInstance,
		},
		OnStartup:     app.startup,
		OnBeforeClose: app.beforeClose,
		OnShutdown:    app.shutdown,
		Bind:          []interface{}{app},
	})
	if err != nil {
		fatal(err)
	}
}

// fatal reports a startup failure the operator can actually see, then exits.
//
// A packaged app has no console: Wails links Windows builds with -H windowsgui,
// and a Finder launch discards stderr. Without the dialog the only symptom of a
// broken build is that double-clicking does nothing at all - the opposite of what
// NewApp's up-front check is for. The log line is what survives to be read later.
func fatal(err error) {
	const title = "CARE Desktop can't start"
	appLog.Writef("FATAL %s: %s", title, err)
	appLog.Close() // os.Exit below skips every defer
	fmt.Fprintln(os.Stderr, title+": "+err.Error())
	switch runtime.GOOS {
	case "darwin":
		_ = proc.Command("osascript", "-e", "display alert "+elevate.OSAQuote(title)+
			" message "+elevate.OSAQuote(err.Error())+" as critical").Run()
	case "windows":
		_ = proc.Command("powershell", "-NoProfile", "-Command",
			"Add-Type -AssemblyName PresentationFramework; [System.Windows.MessageBox]::Show("+
				elevate.PSQuote(err.Error())+","+elevate.PSQuote(title)+")").Run()
	}
	os.Exit(1)
}
