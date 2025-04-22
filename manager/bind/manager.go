package bind

import (
	"github.com/tk103331/mymcp/manager/data"
	"github.com/tk103331/mymcp/manager/manager"
	"github.com/tk103331/mymcp/pkg/common"
)

type Manager struct{}

// StartWorkspace 加载工作空间并启动其中的所有服务
func (m *Manager) StartWorkspace(workspaceID string) error {
	return manager.StartWorkspace(workspaceID)
}

// NewServerInstance 启动单个服务
func (m *Manager) NewServerInstance(cfg *common.ServerConfig) (*data.ServerInstance, error) {
	return manager.NewServerInstance(cfg)
}

// StartServerInstance 启动单个服务
func (m *Manager) StartServerInstance(id string) error {
	return manager.StartServerInstance(id)
}

// StopServerInstance 停止服务
func (m *Manager) StopServerInstance(serverID string) error {
	return manager.StopServerInstance(serverID)
}

// GetServerInstance 获取服务实例
func (m *Manager) GetServerInstance(serverID string) (*data.ServerInstance, error) {
	return manager.GetServerInstance(serverID)
}

// GetWorkspaceServerInstances 获取工作空间关联的服务配置列表
func (d *Manager) GetWorkspaceServerInstances(workspaceID string) ([]*data.ServerInstance, error) {
	servers, err := data.GetWorkspaceServerConfigs(workspaceID)
	if err != nil {
		return nil, err
	}

	var instances []*data.ServerInstance
	for _, config := range servers {
		instance, err := manager.GetServerInstance(config.ID)
		if err != nil || instance == nil {
			instance = &data.ServerInstance{
				ID:     config.ID,
				Config: config,
				Status: "stopped",
				Error:  "error",
			}
		}
		instances = append(instances, instance)
	}
	return instances, nil
}
