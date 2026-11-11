package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/tk103331/mymcp/internal/data"
	"github.com/tk103331/mymcp/internal/model"
	"github.com/tk103331/mymcp/internal/plugin"
)

func TestClientConfigMerge(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".cursor", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"mcpServers":{"keep":{"url":"http://example.com/sse"}},"theme":"dark"}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	client := clientByName("cursor")
	if client == nil {
		t.Fatal("cursor is not a supported client")
	}
	instances := []*data.ServerInstance{
		{
			ID:       "s1",
			Config:   &model.ServerConfig{ID: "s1", Name: "firecrawl"},
			Status:   "started",
			Endpoint: "http://localhost:8421/s1/sse",
		},
	}
	if err := saveManagedClientConfig(client, instances); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Theme      string                    `json:"theme"`
		McpServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	if document.Theme != "dark" {
		t.Errorf("the merge dropped another key: %q", raw)
	}
	if _, ok := document.McpServers["keep"]; !ok {
		t.Errorf("the merge dropped an existing server: %q", raw)
	}
	if got := document.McpServers["firecrawl"]["url"]; got != "http://localhost:8421/s1/sse" {
		t.Errorf("firecrawl is %v, want its endpoint", got)
	}
}

func TestClientConfigShapes(t *testing.T) {
	instances := []*data.ServerInstance{
		{ID: "s1", Config: &model.ServerConfig{ID: "s1", Name: "firecrawl"}, Endpoint: "http://localhost:8421/s1/sse"},
	}
	claude := clientByName("claude").Config(instances)
	server := claude["mcpServers"].(map[string]any)["firecrawl"].(map[string]any)
	if server["command"] != "mcp-pipe" {
		t.Errorf("claude server is %v, want the mcp-pipe bridge", server)
	}
	cherry := clientByName("cherrystudio").Config(instances)
	entry := cherry["mcpServers"].(map[string]any)["firecrawl"].(map[string]any)
	if entry["type"] != "sse" || entry["url"] != "http://localhost:8421/s1/sse" {
		t.Errorf("cherrystudio entry is %v", entry)
	}
}

func TestNewClientFormats(t *testing.T) {
	instances := []*data.ServerInstance{
		{ID: "s1", Config: &model.ServerConfig{ID: "s1", Name: "firecrawl"}, Endpoint: "http://localhost:8421/s1/sse"},
	}
	check := func(name, key, entryType string) {
		t.Helper()
		config := clientByName(name).Config(instances)
		section, ok := config[key].(map[string]any)
		if !ok {
			t.Fatalf("%s has no %s section: %v", name, key, config)
		}
		entry, ok := section["firecrawl"].(map[string]any)
		if !ok {
			t.Fatalf("%s has no firecrawl entry: %v", name, section)
		}
		if entry["url"] != "http://localhost:8421/s1/sse" {
			t.Errorf("%s url is %v", name, entry["url"])
		}
		if entryType != "" && entry["type"] != entryType {
			t.Errorf("%s type is %v, want %s", name, entry["type"], entryType)
		}
	}
	check("claudecode", "mcpServers", "sse")
	check("copilot", "mcpServers", "sse")
	check("qoder", "mcpServers", "sse")
	check("vscode", "servers", "sse")
	check("lmstudio", "mcpServers", "")
	check("gemini", "mcpServers", "")
	check("opencode", "mcp", "remote")
	check("crush", "mcp", "sse")

	cline := clientByName("cline").Config(instances)
	transport := cline["mcpServers"].(map[string]any)["firecrawl"].(map[string]any)["transport"].(map[string]any)
	if transport["type"] != "sse" || transport["url"] != "http://localhost:8421/s1/sse" {
		t.Errorf("cline transport is %v", transport)
	}
}

func TestCodexConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("model = \"o3\"\n\n[mcp_servers.keep]\ncommand = \"keep\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	instances := []*data.ServerInstance{
		{ID: "s1", Config: &model.ServerConfig{ID: "s1", Name: "firecrawl"}, Endpoint: "http://localhost:8421/s1/sse"},
	}
	client := clientByName("codex")
	rendered, err := client.agent.Render(plugin.RenderContext{
		Mode: "direct", Entry: "mymcp", Path: path,
		Servers: clientServers(client.agent, instances),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(rendered.Content), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := toml.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	servers := document["mcp_servers"].(map[string]any)
	if _, ok := servers["keep"]; !ok {
		t.Errorf("the existing server went away: %s", raw)
	}
	if _, ok := servers["firecrawl"]; !ok {
		t.Errorf("firecrawl was not written: %s", raw)
	}
	if document["model"] != "o3" {
		t.Errorf("the existing model went away: %s", raw)
	}
	if !strings.Contains(client.PreviewText(instances), "[mcp_servers.firecrawl]") {
		t.Errorf("the preview is not TOML: %q", client.PreviewText(instances))
	}
}
