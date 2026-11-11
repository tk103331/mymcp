package plugin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPluginRenderAndLoad(t *testing.T) {
	dir := t.TempDir()
	manifest := Manifest{
		ID: "demo-agent", Label: "Demo Agent",
		Paths: map[string]string{"darwin": "~/.demo/mcp.json", "linux": "~/.demo/mcp.json", "windows": "~/.demo/mcp.json"},
	}
	raw, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	source := `
export function render(ctx) {
  const doc = ctx.content ? JSON.parse(ctx.content) : {};
  doc.mode = ctx.mode;
  if (ctx.mode === "router") doc.endpoint = ctx.endpoint;
  return JSON.stringify(doc, null, 2) + "\n";
}
export function load(ctx) {
  return [{name: "demo", transport: "sse", url: "https://example.com/sse"}];
}
`
	if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := p.Render(context.Background(), RenderContext{
		Mode: "router", Endpoint: "http://127.0.0.1:8422/mcp/default",
		Content: `{"keep":true}`, Path: p.Path(), OS: "darwin",
	})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(rendered.Content), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["keep"] != true || doc["endpoint"] != "http://127.0.0.1:8422/mcp/default" {
		t.Fatalf("rendered = %s", rendered.Content)
	}
	servers, err := p.LoadServers(context.Background(), LoadContext{Content: rendered.Content})
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 || servers[0].Name != "demo" || servers[0].Transport != "sse" {
		t.Fatalf("servers = %+v", servers)
	}
}

func TestRejectsPathTraversalIDs(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{".", "..", "../escape", `..\escape`, ""} {
		if err := Remove(root, id); err == nil {
			t.Errorf("Remove(%q) was accepted", id)
		}
	}
	dir := t.TempDir()
	manifest := `{"id":"../escape","label":"bad","paths":{"darwin":"~/x"}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(`export function render(){return ""}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("plugin with a traversal id was loaded")
	}
}

func TestPluginDocumentRender(t *testing.T) {
	dir := t.TempDir()
	manifest := Manifest{
		ID: "doc-agent", Label: "Doc Agent", Format: "json", Section: "mcpServers",
		Paths: map[string]string{"darwin": "~/.doc/mcp.json"},
	}
	raw, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	source := `
export function render(ctx) {
  const doc = ctx.document || {};
  doc.mcpServers ??= {};
  if (ctx.mode === "router") doc.mcpServers[ctx.entry || "mymcp"] = {url: ctx.endpoint, bridge: ctx.bridge};
  for (const name of ctx.remove || []) delete doc.mcpServers[name];
  for (const server of ctx.servers || []) doc.mcpServers[server.key || server.name] = {url: server.url};
  return doc;
}
`
	if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := p.Render(context.Background(), RenderContext{
		Mode: "router", Endpoint: "http://127.0.0.1:8422/mcp/default", Entry: "mymcp",
		Bridge: "/home/me/.mymcp/bin/mcp-pipe", Document: map[string]any{"theme": "dark"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rendered.Content != "" || rendered.Document == nil {
		t.Fatalf("rendered = %+v", rendered)
	}
	servers := rendered.Document["mcpServers"].(map[string]any)
	entry := servers["mymcp"].(map[string]any)
	if entry["url"] != "http://127.0.0.1:8422/mcp/default" || entry["bridge"] != "/home/me/.mymcp/bin/mcp-pipe" {
		t.Fatalf("entry = %v", entry)
	}
	if rendered.Document["theme"] != "dark" {
		t.Fatalf("document = %v", rendered.Document)
	}
}
