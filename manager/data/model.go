package data

import (
	"fmt"
	"net/url"

	"github.com/mark3labs/mcp-go/mcp"
)

type Workspace struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	AutoRun     bool   `json:"autoRun"`
}

type ServerInstance struct {
	ID         string              `json:"id"`
	Config     *ServerConfig       `json:"config"`
	Status     string              `json:"status"`
	Error      string              `json:"error"`
	ServerInfo *mcp.Implementation `json:"serverInfo"`
}

type ServerConfig struct {
	ID        string            `json:"id"`
	Workspace string            `json:"workspace"`
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Transport string            `json:"transport"`
	Cmd       string            `json:"cmd"` // for stdio
	Env       []string          `json:"env"`
	Url       string            `json:"url"`    // for sse
	Params    map[string]string `json:"params"` // for paramaterized cmd or url
}

type Settings struct {
	Language string `json:"language"`
	Theme    string `json:"theme"`
	BaseURL  string `json:"baseUrl"`
}

// ParseBaseURL 从BaseURL中解析出主机和端口
func (s *Settings) ParseBaseURL() (host string, port string, err error) {
	if s.BaseURL == "" {
		return "", "", fmt.Errorf("BaseURL is empty")
	}

	u, err := url.Parse(s.BaseURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid BaseURL: %v", err)
	}

	host = u.Hostname()
	port = u.Port()

	if port == "" {
		switch u.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return "", "", fmt.Errorf("unsupported scheme: %s", u.Scheme)
		}
	}

	return host, port, nil
}
