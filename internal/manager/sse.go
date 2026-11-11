package manager

import (
	"log"
	"net/http"
	"sync"

	"github.com/tk103331/mymcp/internal/data"
	"github.com/tk103331/mymcp/internal/proxy"
)

var (
	httpServer *http.Server
	httpMutex  sync.RWMutex
	sseServers = make(map[string]*proxy.SSEServer) // 使用配置ID作为key
)

// InitHttpServer 读取设置并创建 HTTP 服务器，同时启动设置了自动运行的工作空间。
func InitHttpServer() {
	settings, err := data.LoadSettings()
	if err != nil {
		log.Println("load settings:", err)
		settings = &data.Settings{BaseURL: "http://localhost:8421"}
	}
	log.Println("init http server: " + settings.BaseURL)
	host, port, err := settings.ParseBaseURL()
	if err != nil {
		log.Println("parse base url:", err)
		return
	}

	addr := host
	if port != "80" && port != "443" {
		addr += ":" + port
	}
	httpServer = &http.Server{
		Addr:    addr,
		Handler: http.HandlerFunc(serveMCP),
	}

}

// StartAutoRunWorkspaces starts the workspaces marked to start with MyMCP.
// The app calls it once at launch; restarting the HTTP server alone does not
// start them again.
func StartAutoRunWorkspaces() {
	for _, workspace := range AutoStartWorkspaces() {
		go func(workspace *data.Workspace) {
			log.Println("自动启动工作空间", workspace.Name)
			if err := StartWorkspace(workspace.ID); err != nil {
				log.Println("启动工作空间失败", err)
			}
		}(workspace)
	}
}

// AutoStartWorkspaces lists the workspaces whose services start with MyMCP.
func AutoStartWorkspaces() []*data.Workspace {
	workspaces, err := data.LoadWorkspaces()
	if err != nil {
		return nil
	}
	var auto []*data.Workspace
	for _, workspace := range workspaces {
		if workspace.AutoRun && !workspace.Disabled {
			auto = append(auto, workspace)
		}
	}
	return auto
}

// serveMCP 按路径把请求分发给对应的 SSE 服务。
func serveMCP(w http.ResponseWriter, r *http.Request) {
	httpMutex.RLock()
	var target *proxy.SSEServer
	for _, sseServer := range sseServers {
		if r.URL.Path == sseServer.CompleteSsePath() || r.URL.Path == sseServer.CompleteMessagePath() {
			target = sseServer
			break
		}
	}
	httpMutex.RUnlock()

	if target == nil {
		http.NotFound(w, r)
		return
	}
	target.ServeHTTP(w, r)
}

func AddProxyRoute(proxyServer *proxy.ProxyServer) {
	httpMutex.Lock()
	defer httpMutex.Unlock()

	settings, err := data.LoadSettings()
	if err != nil {
		log.Println("load settings:", err)
		return
	}
	sseServer := proxy.NewSSEServer(proxyServer,
		proxy.WithBaseURL(settings.BaseURL), proxy.WithBasePath(proxyServer.ID),
	)
	sseServers[proxyServer.ID] = sseServer
	log.Printf("Serve %s[%s] on %s\n", proxyServer.ServerInfo, proxyServer.ID, sseServer.CompleteSseEndpoint())
}

func RemoveProxyRoute(proxyServer *proxy.ProxyServer) {
	httpMutex.Lock()
	defer httpMutex.Unlock()
	delete(sseServers, proxyServer.ID)
}

func StartHttpServer() error {
	if httpServer == nil {
		InitHttpServer()
	}
	if httpServer == nil {
		return nil
	}
	log.Println("启动HTTP服务器", httpServer.Addr)
	return httpServer.ListenAndServe()
}

func RestartHttpServer() error {
	if httpServer != nil {
		if err := httpServer.Close(); err != nil {
			return err
		}
	}
	InitHttpServer()
	return StartHttpServer()
}

// StopHttpServer closes the shared workspace HTTP server, if running.
func StopHttpServer() {
	if httpServer != nil {
		_ = httpServer.Close()
	}
}
