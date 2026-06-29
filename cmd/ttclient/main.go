package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Требуется root для TUN
	if os.Geteuid() != 0 {
		log.Println("WARNING: ttclient needs root to create TUN device.")
		log.Println("If Connect fails with permission errors, relaunch with sudo.")
	}

	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "TrustTunnel",
		Width:            420,
		Height:           620,
		MinWidth:         360,
		MinHeight:        560,
		DisableResize:    false,
		BackgroundColour: &options.RGBA{R: 10, G: 10, B: 10, A: 1},

		AssetServer: &assetserver.Options{Assets: assets},

		OnStartup:        app.startup,
		OnDomReady:       app.domReady,
		OnShutdown:       app.shutdown,

		Bind: []interface{}{app},

		Mac: &mac.Options{
			TitleBar:   mac.TitleBarHiddenInset(),
			Appearance: mac.NSAppearanceNameDarkAqua,
			About: &mac.AboutInfo{
				Title:   "TrustTunnel",
				Message: "Desktop client for TrustTunnel endpoint",
			},
		},
	})

	if err != nil {
		log.Fatalln("wails:", err)
	}
}
