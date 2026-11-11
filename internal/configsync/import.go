package configsync

import (
	"context"
	"fmt"
	"strings"
)

// ImportedServer is one MCP server read from a client's native config.
type ImportedServer struct {
	Client    string            `json:"client"`
	Name      string            `json:"name"`
	Transport string            `json:"transport"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	URL       string            `json:"url,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

// Import reads one client's servers, leaving out entries MyMCP wrote.
func (m *Manager) Import(clientID string) ([]ImportedServer, error) {
	client := m.clientByID(clientID)
	if client == nil {
		return nil, fmt.Errorf("unknown client %q", clientID)
	}
	if client.Path() == "" {
		return nil, nil
	}
	servers, err := client.Load(context.Background())
	if err != nil {
		return nil, err
	}
	managed := map[string]bool{}
	if record, ok := m.state.Clients[client.ID()]; ok {
		if record.Mode == "direct" || record.Mode == "plugin-direct" {
			for name := range record.Entries {
				managed[name] = true
			}
		}
		if record.Entry != "" {
			managed[record.Entry] = true
		}
	}
	var out []ImportedServer
	for _, server := range servers {
		if managed[server.Name] {
			continue
		}
		imported := ImportedServer{
			Client: client.ID(), Name: server.Name, Transport: server.Transport,
			Command: server.Command, Args: server.Args, URL: server.URL,
			Env: server.Env, Headers: server.Headers,
		}
		if !mymcpRouterEntry(imported) {
			out = append(out, imported)
		}
	}
	return out, nil
}

// ImportAll reads every supported client's file.
func (m *Manager) ImportAll() ([]ImportedServer, error) {
	var out []ImportedServer
	for _, client := range m.agents.Agents() {
		servers, err := m.Import(client.ID())
		if err != nil {
			return nil, err
		}
		out = append(out, servers...)
	}
	return out, nil
}

func mymcpRouterEntry(server ImportedServer) bool {
	if server.Name == "mymcp" && (strings.Contains(server.URL, "/mcp/") || server.Command == "mcp-pipe") {
		return true
	}
	if server.Command == "mcp-pipe" && strings.Contains(strings.Join(server.Args, " "), "/mcp/") {
		return true
	}
	return false
}
