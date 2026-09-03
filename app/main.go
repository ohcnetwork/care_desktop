package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// kitFS holds the deployment kit (compose + env + mounted configs), unpacked to a
// writable dir on first run. Staged into ./kit by the frontend build step.
//
//go:embed all:kit
var kitFS embed.FS

func main() {
	app := NewApp(kitFS)

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
