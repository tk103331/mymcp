package data

import (
	"fmt"
	"net/url"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tk103331/mymcp/internal/model"
)

type Workspace struct {
	ID             string                   `json:"id"`
	Name           string                   `json:"name"`
	Status         string                   `json:"status"`
	Description    string                   `json:"description"`
	Enabled        bool                     `json:"enabled"` // kept for older files
	Disabled       bool                     `json:"disabled,omitempty"`
	AutoRun        bool                     `json:"autoRun"`
	ManagedClients map[string]ManagedClient `json:"managedClients"`
}

type ManagedClient struct {
	Config string `json:"config"`
}

type ServerInstance struct {
	ID         string              `json:"id"`
	Config     *model.ServerConfig `json:"config"`
	Status     string              `json:"status"`
	Error      string              `json:"error"`
	ServerInfo *mcp.Implementation `json:"serverInfo"`
	Endpoint   string              `json:"endpoint"`
}

type Settings struct {
	Language      string `json:"language"`
	Theme         string `json:"theme"`
	BaseURL       string `json:"baseUrl"`
	GatewayAddr   string `json:"gatewayAddr"`
	LaunchAtLogin bool   `json:"launchAtLogin"`
}

// ParseBaseURL 从 BaseURL 中解析出主机和端口
func (s *Settings) ParseBaseURL() (host string, port string, err error) {
	if s.BaseURL == "" {
		return "", "", fmt.Errorf("BaseURL is empty")
	}

	u, err := url.Parse(s.BaseURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid BaseURL: %v", err)
	}

	return u.Hostname(), u.Port(), nil
}
