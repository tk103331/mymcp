package manager

import (
	"log"
	"net/http"
	"sync"

	"github.com/tk103331/mymcp/manager/data"
	"github.com/tk103331/mymcp/pkg/proxy"

	"github.com/gorilla/mux"
)

var (
	httpServer *http.Server
	httpRouter *mux.Router
	httpMutex  sync.Mutex
	sseServers = make(map[string]*proxy.SSEServer) // 使用配置ID作为key
)

func InitHttpServer() {
	r := mux.NewRouter()

	// 从settings.json加载配置
	settings, err := data.LoadSettings()
	if err != nil {
		return
	}
	log.Println("init http server: " + settings.BaseURL)
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
		if !workspace.Enabled || !workspace.AutoRun {
			continue
		}
		go func() {
			// 启动工作空间
			log.Println("自动启动工作空间", workspace.Name)
			if err := StartWorkspace(workspace.ID); err != nil {
				log.Println("启动工作空间失败", err)
			}
		}()
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
	basePath := proxyServer.ID
	sseServer := proxy.NewSSEServer(proxyServer,
		proxy.WithBaseURL(settings.BaseURL), proxy.WithBasePath(basePath),
	)

	// 保存SSE服务器
	sseServers[proxyServer.ID] = sseServer

	httpRouter.Handle(sseServer.CompleteSsePath(), sseServer)
	httpRouter.Handle(sseServer.CompleteMessagePath(), sseServer)

	log.Printf("Serve %s[%s] on %s\n", proxyServer.ServerInfo, proxyServer.ID, sseServer.CompleteSseEndpoint())
}

func RemoveProxyRoute(proxyServer *proxy.ProxyServer) {
	httpMutex.Lock()
	defer httpMutex.Unlock()

	sseServer, ok := sseServers[proxyServer.ID]
	if !ok {
		return
	}
	// 移除ProxyRoute
	httpRouter.Handle(sseServer.CompleteSsePath(), nil)
	httpRouter.Handle(sseServer.CompleteMessagePath(), nil)
	// 删除SSE服务器
	delete(sseServers, proxyServer.ID)
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
