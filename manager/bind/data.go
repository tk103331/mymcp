package bind

import (
	"github.com/tk103331/mymcp/manager/data"
)

type Data struct {
}

// SaveWorkspace 保存工作空间配置到文件
func (d *Data) SaveWorkspace(workspace *data.Workspace) error {
	return data.SaveWorkspace(workspace)
}

// LoadWorkspaces 从文件加载工作空间配置
func (d *Data) LoadWorkspaces() ([]*data.Workspace, error) {
	return data.LoadWorkspaces()
}

// DeleteWorkspace 删除工作空间
func (d *Data) DeleteWorkspace(id string) error {
	return data.DeleteWorkspace(id)
}

// SaveServerConfig 保存服务配置到文件
func (d *Data) SaveServerConfig(config *data.ServerConfig) error {
	return data.SaveServerConfig(config)
}

// LoadServerConfigs 从文件加载服务配置
func (d *Data) LoadServerConfigs() ([]*data.ServerConfig, error) {
	return data.LoadServerConfigs()
}

// GetWorkspace 通过ID获取工作空间信息
func (d *Data) GetWorkspace(id string) (*data.Workspace, error) {
	return data.GetWorkspace(id)
}

// DeleteServerConfig 删除服务配置
func (d *Data) DeleteServerConfig(id string) error {
	return data.DeleteServerConfig(id)
}
