package main

import (
	"context"

	"github.com/labstack/gommon/log"
	"github.com/tk103331/mymcp/manager/manager"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// 初始化SSE服务器
	go func() {
		// 重新初始化服务器
		manager.InitHttpServer()

		// 启动新服务器
		err := manager.StartHttpServer()
		if err != nil {
			log.Warnf("启动服务器错误：%s", err.Error())
			return
		}
	}()
}
