package main

import (
	"embed"

	"mcphosting/manager/bind"
	"mcphosting/manager/manager"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// 初始化SSE服务器
	manager.InitHttpServer()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "My MCP",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			&bind.Data{},
			&bind.Manager{},
			&bind.Setting{},
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
