package configsync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPluginClient(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".mymcp")
	pluginDir := filepath.Join(dir, "plugins", "demo")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"id":"demo-agent","label":"Demo Agent","paths":{"darwin":"~/.demo/mcp.json","linux":"~/.demo/mcp.json","windows":"~/.demo/mcp.json"}}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	source := `
export function render(ctx) {
  const doc = ctx.content ? JSON.parse(ctx.content) : {};
  doc.mcpServers ??= {};
  if (ctx.mode === "detach") delete doc.mcpServers.mymcp;
  else if (ctx.mode === "router") doc.mcpServers.mymcp = {url: ctx.endpoint};
  else for (const s of ctx.servers) doc.mcpServers[s.name] = {command: s.command, args: s.args};
  return JSON.stringify(doc, null, 2) + "\n";
}
export function load(ctx) {
  return [{name: "demo", transport: "sse", url: "https://example.com/sse"}];
}
`
	if err := os.WriteFile(filepath.Join(pluginDir, "index.js"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	statuses := manager.Statuses()
	found := false
	for _, status := range statuses {
		if status.Client == "demo-agent" {
			found = true
		}
	}
	if !found {
		t.Fatalf("plugin client missing: %+v", statuses)
	}
	if _, err := manager.Apply("demo-agent", "http://127.0.0.1:8422/mcp/default", "mymcp", false); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(home, ".demo", "mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	servers := doc["mcpServers"].(map[string]any)
	entry := servers["mymcp"].(map[string]any)
	if entry["url"] != "http://127.0.0.1:8422/mcp/default" {
		t.Fatalf("entry = %v", entry)
	}
	imported, err := manager.Import("demo-agent")
	if err != nil {
		t.Fatal(err)
	}
	if len(imported) != 1 || imported[0].Name != "demo" || imported[0].Client != "demo-agent" {
		t.Fatalf("imported = %+v", imported)
	}
	if err := manager.Detach("demo-agent", false); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(filepath.Join(home, ".demo", "mcp.json"))
	_ = json.Unmarshal(raw, &doc)
	servers = doc["mcpServers"].(map[string]any)
	if _, ok := servers["mymcp"]; ok {
		t.Fatalf("detach left the entry: %s", raw)
	}
}
