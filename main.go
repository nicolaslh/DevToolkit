package main

import (
	"embed"
	_ "embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := application.New(application.Options{
		Name:        "DevToolkit",
		Description: "桌面级开发者工具箱",
		Services: []application.Service{
			application.NewService(&CodecService{}),
			application.NewService(&SecurityService{}),
			application.NewService(&NetworkService{}),
			application.NewService(&FrontendService{}),
			application.NewService(&TextService{}),
			application.NewService(&DevOpsService{}),
			application.NewService(&MediaService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "DevToolkit",
		Width:     1100,
		Height:    720,
		MinWidth:  880,
		MinHeight: 560,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropNormal,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(255, 255, 255),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
