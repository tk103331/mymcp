package manager

import (
	"log"
	"mcphosting/manager/data"
	"mcphosting/manager/proxy"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
)

var (
	httpServer *http.Server
	httpRouter *mux.Router
	httpMutex  sync.Mutex
)

func InitHttpServer() {
	r := mux.NewRouter()

	// 从settings.json加载配置
	settings, err := data.LoadSettings()
	if err != nil {
		return
	}

	host, port, err := settings.ParseBaseURL()
	if err != nil {
		return
	}

	addr := host
	if port != "80" && port != "443" {
		addr += ":" + port
	}
	httpServer = &http.Server{
		Addr:    addr,
		Handler: r,
	}
	httpRouter = r

	// 如果设置了autoRun，自动启动工作空间服务
	workspaces, err := data.LoadWorkspaces()
	if err != nil {
		return
	}

	for _, workspace := range workspaces {
		if !workspace.AutoRun {
			continue
		}
		// 启动工作空间
		log.Println("自动启动工作空间", workspace.Name)
		if err := StartWorkspace(workspace.ID); err != nil {
			continue
		}

		// 为每个服务实例添加ProxyRoute
		for _, instance := range instances {
			log.Println("自动添加ProxyRoute", instance.Config.Name)
			AddProxyRoute(instance)
		}
	}
}

func AddProxyRoute(proxyServer *proxy.ProxyServer) {
	httpMutex.Lock()
	defer httpMutex.Unlock()

	// 从settings.json加载配置
	settings, err := data.LoadSettings()
	if err != nil {
		return
	}

	sseServer := proxy.NewSSEServer(proxyServer,
		proxy.WithBaseURL(settings.BaseURL), proxy.WithBasePath(""),
	)
	httpRouter.Handle(sseServer.CompleteSsePath(), sseServer)
	httpRouter.Handle(sseServer.CompleteMessagePath(), sseServer)
}

func StartHttpServer() error {
	log.Println("启动HTTP服务器", httpServer.Addr)
	return httpServer.ListenAndServe()
}

func RestartHttpServer() error {
	httpMutex.Lock()
	defer httpMutex.Unlock()

	// 关闭现有服务器
	if httpServer != nil {
		if err := httpServer.Close(); err != nil {
			return err
		}
	}

	// 重新初始化服务器
	InitHttpServer()

	// 启动新服务器
	return StartHttpServer()
}
