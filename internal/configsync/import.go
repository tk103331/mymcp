package configsync

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/tk103331/mymcp/internal/plugin"
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
	if client.Plugin != nil {
		raw, err := os.ReadFile(client.Path())
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		servers, err := client.Plugin.LoadServers(context.Background(), plugin.LoadContext{
			Content: string(raw), Path: client.Path(), OS: runtime.GOOS,
		})
		if err != nil {
			return nil, err
		}
		var out []ImportedServer
		for _, server := range servers {
			imported := ImportedServer{
				Client: client.ID, Name: server.Name, Transport: server.Transport,
				Command: server.Command, Args: server.Args, URL: server.URL,
				Env: server.Env, Headers: server.Headers,
			}
			if !mymcpRouterEntry(imported) {
				out = append(out, imported)
			}
		}
		return out, nil
	}
	doc, err := readDocument(client, client.Path())
	if err != nil {
		return nil, err
	}
	managed := map[string]bool{}
	if record, ok := m.state.Clients[client.ID]; ok {
		if record.Mode == "direct" {
			for name := range record.Entries {
				managed[name] = true
			}
		} else if record.Entry != "" {
			managed[record.Entry] = true
		}
	}
	section := sectionOf(doc, client.Root)
	var out []ImportedServer
	for name, value := range section {
		if managed[name] {
			continue
		}
		entry, ok := value.(map[string]any)
		if !ok {
			continue
		}
		server, ok := parseEntry(client.ID, name, entry)
		if !ok || mymcpRouterEntry(server) {
			continue
		}
		out = append(out, server)
	}
	return out, nil
}

// ImportAll reads every supported client's file.
func (m *Manager) ImportAll() ([]ImportedServer, error) {
	var out []ImportedServer
	for _, client := range m.clients() {
		servers, err := m.Import(client.ID)
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

func parseEntry(clientID, name string, entry map[string]any) (ImportedServer, bool) {
	server := ImportedServer{Client: clientID, Name: name}
	// Cline nests the transport.
	if transport, ok := entry["transport"].(map[string]any); ok {
		entry = transport
	}
	if strings.EqualFold(str(entry["type"]), "remote") { // OpenCode
		server.Transport = "sse"
		server.URL = str(entry["url"])
		server.Headers = strMap(entry["headers"])
		return server, server.URL != ""
	}
	if strings.EqualFold(str(entry["type"]), "local") { // OpenCode
		command := strList(entry["command"])
		if len(command) == 0 {
			return server, false
		}
		server.Transport, server.Command, server.Args = "stdio", command[0], command[1:]
		server.Env = strMap(entry["environment"])
		return server, true
	}
	kind := strings.ToLower(str(entry["type"]))
	if url := firstNonEmpty(str(entry["serverUrl"]), str(entry["httpUrl"]), str(entry["url"]), str(entry["uri"])); url != "" {
		server.URL = url
		switch {
		case kind == "http" || kind == "streamablehttp" || kind == "streamable-http" || str(entry["httpUrl"]) != "":
			server.Transport = "http"
		case kind == "sse" || kind == "streamable_http":
			server.Transport = "sse"
		case clientID == "zed" || clientID == "codex":
			server.Transport = "http"
		default:
			server.Transport = "sse"
		}
		server.Headers = strMap(entry["headers"])
		return server, true
	}
	command := str(entry["command"])
	args := strList(entry["args"])
	if command == "" {
		command = str(entry["cmd"])
		if command == "" {
			return server, false
		}
	}
	if values := strList(entry["command"]); len(values) > 0 { // rare list shape
		command, args = values[0], values[1:]
	}
	server.Transport = "stdio"
	server.Command, server.Args = command, args
	server.Env = strMap(entry["env"])
	if server.Env == nil {
		server.Env = strMap(entry["envs"])
	}
	if server.Env == nil {
		server.Env = strMap(entry["environment"])
	}
	return server, true
}

func str(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprint(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func strList(value any) []string {
	switch v := value.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, str(item))
		}
		return out
	case []string:
		return v
	}
	return nil
}

func strMap(value any) map[string]string {
	raw, ok := value.(map[string]any)
	if !ok {
		if direct, ok := value.(map[string]string); ok {
			return direct
		}
		return nil
	}
	out := make(map[string]string, len(raw))
	for key, item := range raw {
		out[key] = str(item)
	}
	return out
}
