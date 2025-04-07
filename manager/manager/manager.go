package manager

import (
	"fmt"
	"sync"

	"mcphosting/manager/data"
	"mcphosting/manager/proxy"

	"github.com/google/uuid"
)

var (
	instanceMutex sync.RWMutex
	instances     = make(map[string]*proxy.ProxyServer) // 使用配置ID作为key
)

// StartWorkspace 加载工作空间并启动其中的所有服务
func StartWorkspace(workspaceID string) error {
	// 获取工作空间信息
	workspace, err := data.GetWorkspace(workspaceID)
	if err != nil {
		return fmt.Errorf("获取工作空间失败: %v", err)
	}
	fmt.Printf("Start workspace %s\n", workspace.Name)

	// 获取工作空间下的所有服务配置
	configs, err := data.GetWorkspaceServerConfigs(workspaceID)
	if err != nil {
		return fmt.Errorf("获取服务配置失败: %v", err)
	}

	// 启动所有服务
	for _, cfg := range configs {
		if _, err := NewServerInstance(cfg); err != nil {
			return fmt.Errorf("启动服务[%s]失败: %v", cfg.Name, err)
		}
	}

	return nil
}

// NewServerInstance 启动单个服务
func NewServerInstance(cfg *data.ServerConfig) (*data.ServerInstance, error) {
	instanceMutex.Lock()
	defer instanceMutex.Unlock()

	instanceId := uuid.New().String()

	// 检查服务是否已存在
	if _, exists := instances[instanceId]; exists {
		return nil, fmt.Errorf("服务[%s]已在运行", cfg.Name)
	}

	// 创建新的代理服务器
	server, err := proxy.NewProxyServer(cfg)
	if err != nil {
		return nil, err
	}

	// 保存服务实例
	instances[instanceId] = server
	return &data.ServerInstance{
		ID:         instanceId,
		Config:     cfg,
		ServerInfo: &server.ServerInfo,
	}, nil
}

// NewServerStartServerInstanceInstance 启动单个服务
func StartServerInstance(instanceID string) error {
	instanceMutex.Lock()
	defer instanceMutex.Unlock()
	fmt.Println("Start server instance", instanceID)
	// 检查服务是否已存在
	if _, exists := instances[instanceID]; exists {
		return fmt.Errorf("服务[%s]已在运行", instanceID)
	}

	// 获取服务配置
	cfg, err := data.GetServerConfig(instanceID)
	if err != nil {
		return fmt.Errorf("获取服务配置失败: %v", err)
	}

	// 创建新的代理服务器
	server, err := proxy.NewProxyServer(cfg)
	if err != nil {
		return err
	}

	// 保存服务实例
	instances[cfg.ID] = server
	fmt.Println("Start server instance", instanceID, "success")
	return nil
}

// StopServerInstance 停止服务
func StopServerInstance(instanceID string) error {
	instanceMutex.Lock()
	defer instanceMutex.Unlock()
	fmt.Println("Stop server instance", instanceID)
	server, exists := instances[instanceID]
	if !exists {
		return fmt.Errorf("服务不存在")
	}

	// 关闭服务连接
	if err := server.Client.Close(); err != nil {
		return fmt.Errorf("关闭服务失败: %v", err)
	}

	// 移除服务实例
	delete(instances, instanceID)
	fmt.Println("Stop server instance", instanceID, "success")
	return nil
}

// GetServerInstance 获取服务实例
func GetServerInstance(instanceID string) (*data.ServerInstance, error) {
	instanceMutex.RLock()
	defer instanceMutex.RUnlock()

	server, exists := instances[instanceID]
	if !exists {
		return nil, fmt.Errorf("服务不存在")
	}
	return &data.ServerInstance{
		ID:         instanceID,
		Config:     server.Config,
		Status:     server.Status,
		ServerInfo: &server.ServerInfo,
	}, nil
}
