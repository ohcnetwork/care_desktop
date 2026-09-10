package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// installFS holds the deployment install dir (compose + env + mounted configs), unpacked to a
// writable dir on first run. Staged into ./install dir by the frontend build step.
//
//go:embed all:install
var installFS embed.FS

func main() {
	app := NewApp(installFS)

	err := wails.Run(&options.App{
		Title:     "CARE Desktop",
		Width:     1180,
		Height:    900,
		MinWidth:  720,
		MinHeight: 560,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind:       []interface{}{app},
	})
	if err != nil {
		println("error:", err.Error())
	}
}
