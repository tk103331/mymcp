package manager

import (
	"fmt"
	"log"
	"sync"

	"github.com/tk103331/mymcp/internal/data"
	"github.com/tk103331/mymcp/internal/model"
	"github.com/tk103331/mymcp/internal/proxy"
)

var (
	proxyMutex   sync.RWMutex
	proxyServers = make(map[string]*proxy.ProxyServer) // 使用配置 ID 作为 key
)

// StartWorkspace 加载工作空间并启动其中的所有服务。
func StartWorkspace(workspaceID string) error {
	workspace, err := data.GetWorkspace(workspaceID)
	if err != nil {
		return fmt.Errorf("获取工作空间失败: %v", err)
	}
	if workspace.Disabled {
		return fmt.Errorf("工作空间[%s]已停用", workspace.Name)
	}
	log.Printf("start workspace %s", workspace.Name)

	configs, err := data.GetWorkspaceServerConfigs(workspaceID)
	if err != nil {
		return fmt.Errorf("获取服务配置失败: %v", err)
	}

	for _, cfg := range configs {
		if _, err := NewServerInstance(cfg); err != nil {
			log.Printf("启动服务[%s]失败: %v", cfg.Name, err)
		}
	}
	return nil
}

// NewServerInstance 启动单个服务。
func NewServerInstance(cfg *model.ServerConfig) (*data.ServerInstance, error) {
	proxyMutex.Lock()
	defer proxyMutex.Unlock()

	if _, exists := proxyServers[cfg.ID]; exists {
		return nil, fmt.Errorf("服务[%s]已在运行", cfg.Name)
	}

	server, err := proxy.NewProxyServer(cfg)
	if err != nil {
		return nil, err
	}
	proxyServers[cfg.ID] = server
	AddProxyRoute(server)

	return &data.ServerInstance{
		ID:         cfg.ID,
		Config:     cfg,
		ServerInfo: &server.ServerInfo,
		Status:     server.Status,
		Error:      server.Error,
		Endpoint:   endpoint(cfg.ID),
	}, nil
}

// StartServerInstance 按配置 ID 启动单个服务。
func StartServerInstance(instanceID string) error {
	proxyMutex.Lock()
	defer proxyMutex.Unlock()

	if _, exists := proxyServers[instanceID]; exists {
		return fmt.Errorf("服务[%s]已在运行", instanceID)
	}

	cfg, err := data.GetServerConfig(instanceID)
	if err != nil {
		return fmt.Errorf("获取服务配置失败: %v", err)
	}
	if workspace, err := data.GetWorkspace(cfg.Workspace); err == nil && workspace.Disabled {
		return fmt.Errorf("工作空间[%s]已停用", workspace.Name)
	}

	server, err := proxy.NewProxyServer(cfg)
	if err != nil {
		return err
	}
	proxyServers[cfg.ID] = server
	AddProxyRoute(server)
	log.Println("start server instance", instanceID, "success")
	return nil
}

// StopServerInstance 停止服务。
func StopServerInstance(instanceID string) error {
	proxyMutex.Lock()
	defer proxyMutex.Unlock()

	server, exists := proxyServers[instanceID]
	if !exists {
		return fmt.Errorf("服务不存在")
	}
	if server.Client != nil {
		if err := server.Client.Close(); err != nil {
			return fmt.Errorf("关闭服务失败: %v", err)
		}
	}
	RemoveProxyRoute(server)
	delete(proxyServers, instanceID)
	log.Println("stop server instance", instanceID, "success")
	return nil
}

// StopWorkspace stops every running service of a workspace.
func StopWorkspace(workspaceID string) error {
	configs, err := data.GetWorkspaceServerConfigs(workspaceID)
	if err != nil {
		return err
	}
	for _, cfg := range configs {
		if IsRunning(cfg.ID) {
			if err := StopServerInstance(cfg.ID); err != nil {
				log.Printf("停止服务[%s]失败: %v", cfg.Name, err)
			}
		}
	}
	return nil
}

// GetServerInstance 获取服务实例。
func GetServerInstance(instanceID string) (*data.ServerInstance, error) {
	proxyMutex.RLock()
	defer proxyMutex.RUnlock()

	instance := &data.ServerInstance{ID: instanceID}
	server, exists := proxyServers[instanceID]
	if !exists {
		return instance, nil
	}
	instance.Config = server.Config
	instance.ServerInfo = &server.ServerInfo
	instance.Status = server.Status
	instance.Error = server.Error
	instance.Endpoint = endpoint(instanceID)
	return instance, nil
}

// IsRunning 报告服务是否正在运行。
func IsRunning(instanceID string) bool {
	proxyMutex.RLock()
	defer proxyMutex.RUnlock()
	_, exists := proxyServers[instanceID]
	return exists
}

func endpoint(instanceID string) string {
	httpMutex.RLock()
	defer httpMutex.RUnlock()
	sseServer, exists := sseServers[instanceID]
	if !exists {
		return ""
	}
	return sseServer.CompleteSseEndpoint()
}

// StopAll stops every running service.
func StopAll() {
	proxyMutex.RLock()
	ids := make([]string, 0, len(proxyServers))
	for id := range proxyServers {
		ids = append(ids, id)
	}
	proxyMutex.RUnlock()
	for _, id := range ids {
		if err := StopServerInstance(id); err != nil {
			log.Printf("stop service %s: %v", id, err)
		}
	}
}
