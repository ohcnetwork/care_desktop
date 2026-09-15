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

var version = "dev"

var appLog *applog.Logger

//go:embed all:frontend/dist
var assets embed.FS

//go:embed all:install
var installFS embed.FS

func main() {
	appLog = applog.Open()
	defer appLog.Close()
	appLog.OnFatal = func(msg string) { fatal(errors.New(msg)) }

	app, err := NewApp(installFS, appLog)
	if err != nil {
		fatal(err)
	}

	appLog.Header(app.pins.AppVersion, app.installDir(), app.loadConfig().MDNSName)
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

func fatal(err error) {
	const title = "CARE Desktop can't start"
	appLog.Writef("FATAL %s: %s", title, err)
	appLog.Close()
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
