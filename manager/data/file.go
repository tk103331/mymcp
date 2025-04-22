package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/tk103331/mymcp/pkg/common"
)

const (
	workspaceConfigFile = "workspaces.json"
	serverConfigFile    = "servers.json"
	settingsConfigFile  = "settings.json"
)

// SaveWorkspace 保存工作空间配置到文件
func SaveWorkspace(workspace *Workspace) error {
	if workspace.ID == "" {
		workspace.ID = uuid.New().String()
	}

	// 读取现有配置
	workspaces, err := LoadWorkspaces()
	if err != nil {
		workspaces = make([]*Workspace, 0)
	}

	// 更新或添加工作空间
	updated := false
	for i, w := range workspaces {
		if w.ID == workspace.ID {
			workspaces[i] = workspace
			updated = true
			break
		}
	}
	if !updated {
		workspaces = append(workspaces, workspace)
	}

	// 保存到文件
	return saveToFile(workspaceConfigFile, workspaces)
}

// LoadWorkspaces 从文件加载工作空间配置
func LoadWorkspaces() ([]*Workspace, error) {
	var workspaces []*Workspace
	err := loadFromFile(workspaceConfigFile, &workspaces)
	if err != nil {
		return nil, err
	}
	return workspaces, nil
}

// DeleteWorkspace 删除工作空间配置
func DeleteWorkspace(id string) error {
	// 读取现有配置
	workspaces, err := LoadWorkspaces()
	if err != nil {
		return err
	}

	// 查找并删除工作空间
	for i, w := range workspaces {
		if w.ID == id {
			workspaces = append(workspaces[:i], workspaces[i+1:]...)
			break
		}
	}

	// 保存到文件
	return saveToFile(workspaceConfigFile, workspaces)
}

// SaveServerConfig 保存服务配置到文件
func SaveServerConfig(config *common.ServerConfig) error {
	if config.ID == "" {
		config.ID = uuid.New().String()
	}

	// 读取现有配置
	configs, err := LoadServerConfigs()
	if err != nil {
		configs = make([]*common.ServerConfig, 0)
	}

	// 更新或添加配置
	updated := false
	for i, c := range configs {
		if c.ID == config.ID {
			configs[i] = config
			updated = true
			break
		}
	}
	if !updated {
		configs = append(configs, config)
	}

	// 保存到文件
	return saveToFile(serverConfigFile, configs)
}

// LoadServerConfigs 从文件加载服务配置
func LoadServerConfigs() ([]*common.ServerConfig, error) {
	var configs []*common.ServerConfig
	err := loadFromFile(serverConfigFile, &configs)
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// DeleteServerConfig 删除服务配置
func DeleteServerConfig(id string) error {
	// 读取现有配置
	configs, err := LoadServerConfigs()
	if err != nil {
		return err
	}

	// 查找并删除服务配置
	for i, c := range configs {
		if c.ID == id {
			configs = append(configs[:i], configs[i+1:]...)
			break
		}
	}

	// 保存到文件
	return saveToFile(serverConfigFile, configs)
}

// saveToFile 保存数据到JSON文件
func saveToFile(filename string, data interface{}) error {
	// 获取配置文件目录
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	// 确保目录存在
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %v", err)
	}

	// 序列化数据
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化数据失败: %v", err)
	}

	// 写入文件
	filePath := filepath.Join(configDir, filename)
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	return nil
}

// loadFromFile 从JSON文件加载数据
func loadFromFile(filename string, data interface{}) error {
	// 获取配置文件目录
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	// 读取文件
	filePath := filepath.Join(configDir, filename)
	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在时返回空数据
		}
		return fmt.Errorf("读取文件失败: %v", err)
	}

	// 解析JSON数据
	if err := json.Unmarshal(jsonData, data); err != nil {
		return fmt.Errorf("解析JSON数据失败: %v", err)
	}

	return nil
}

// getConfigDir 获取配置文件目录
func getConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户目录失败: %v", err)
	}
	return filepath.Join(homeDir, ".mymcp"), nil
}

// GetWorkspace 通过ID获取工作空间信息
func GetWorkspace(id string) (*Workspace, error) {
	workspaces, err := LoadWorkspaces()
	if err != nil {
		return nil, err
	}

	for _, w := range workspaces {
		if w.ID == id {
			return w, nil
		}
	}

	return nil, fmt.Errorf("工作空间不存在: %s", id)
}

// GetWorkspaceServerConfigs 获取工作空间关联的服务配置列表
func GetWorkspaceServerConfigs(workspaceID string) ([]*common.ServerConfig, error) {
	// 先验证工作空间是否存在
	_, err := GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	// 加载所有服务配置
	configs, err := LoadServerConfigs()
	if err != nil {
		return nil, err
	}

	// 过滤出属于该工作空间的服务配置
	workspaceConfigs := make([]*common.ServerConfig, 0)
	for _, config := range configs {
		if config.Workspace == workspaceID {
			workspaceConfigs = append(workspaceConfigs, config)
		}
	}

	return workspaceConfigs, nil
}

// GetServerConfig 通过ID获取服务配置信息
func GetServerConfig(id string) (*common.ServerConfig, error) {
	configs, err := LoadServerConfigs()
	if err != nil {
		return nil, err
	}

	for _, c := range configs {
		if c.ID == id {
			return c, nil
		}
	}

	return nil, fmt.Errorf("服务配置不存在: %s", id)
}

// SaveSettings 保存设置到文件
func SaveSettings(settings *Settings) error {
	// 保存到文件
	return saveToFile(settingsConfigFile, settings)
}

// LoadSettings 从文件加载设置
func LoadSettings() (*Settings, error) {
	var settings Settings
	err := loadFromFile(settingsConfigFile, &settings)
	if err != nil {
		return nil, err
	}
	if settings.Language == "" {
		settings.Language = "zh"
	}
	if settings.Theme == "" {
		settings.Theme = "light"
	}
	if settings.BaseURL == "" {
		settings.BaseURL = "http://localhost:8421"
	}
	return &settings, nil
}
