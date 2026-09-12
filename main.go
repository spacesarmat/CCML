package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

// frontendAssets contains the production React bundle built by Wails/Vite.
//
//go:embed all:frontend/dist
var frontendAssets embed.FS

func main() {
	app, err := NewApp()
	if err != nil {
		log.Fatalf("initialize application: %v", err)
	}
	defer func() {
		if err := app.Close(); err != nil {
			log.Printf("close application: %v", err)
		}
	}()

	err = wails.Run(&options.App{
		Title:            "CCML Music Library",
		Width:            1320,
		Height:           860,
		MinWidth:         1000,
		MinHeight:        650,
		AssetServer:      &assetserver.Options{Assets: frontendAssets},
		BackgroundColour: &options.RGBA{R: 19, G: 23, B: 31, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatalf("run Wails application: %v", err)
	}
}
