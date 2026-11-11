package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/tk103331/mymcp/internal/plugin"
	"gopkg.in/yaml.v3"
)

func TestBuiltinAgents(t *testing.T) {
	builtins, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	if len(builtins) != 24 {
		t.Fatalf("got %d built-in agents, want 24", len(builtins))
	}
	ids := map[string]bool{}
	for _, a := range builtins {
		ids[a.ID()] = true
	}
	for _, id := range []string{"cursor", "claude", "codex", "goose", "deepchat", "chatwise", "cherrystudio", "antigravity"} {
		if !ids[id] {
			t.Errorf("missing built-in agent %q", id)
		}
	}
}

func TestBuiltinRenderRouter(t *testing.T) {
	builtins, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range builtins {
		t.Run(a.ID(), func(t *testing.T) {
			rendered, err := a.Render(plugin.RenderContext{
				Mode: "router", Endpoint: "http://127.0.0.1:8422/mcp/default", Entry: "mymcp",
				Bridge: "/home/me/.mymcp/bin/mcp-pipe", Document: map[string]any{},
			})
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(rendered.Content) == "" {
				t.Fatal("empty content")
			}
			doc := rendered.Document
			if doc == nil {
				switch a.Format() {
				case "toml":
					var parsed map[string]any
					if err := toml.Unmarshal([]byte(rendered.Content), &parsed); err != nil {
						t.Fatal(err)
					}
					doc = parsed
				case "yaml":
					var parsed map[string]any
					if err := yaml.Unmarshal([]byte(rendered.Content), &parsed); err != nil {
						t.Fatal(err)
					}
					doc = parsed
				default:
					if err := json.Unmarshal([]byte(rendered.Content), &doc); err != nil {
						t.Fatal(err)
					}
				}
			}
			section := doc[a.Section()].(map[string]any)
			if section["mymcp"] == nil {
				t.Fatalf("mymcp entry missing from %s: %v", a.Section(), doc)
			}
		})
	}
}

func TestBuiltinDirectAndLoad(t *testing.T) {
	builtins, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range builtins {
		if a.Path() == "" {
			continue
		}
		t.Run(a.ID(), func(t *testing.T) {
			rendered, err := a.Render(plugin.RenderContext{
				Mode: "direct", Entry: "mymcp", Bridge: "mcp-pipe", Document: map[string]any{},
				Servers: []plugin.Server{
					{Name: "local", Key: a.Key("local", ""), Transport: "stdio", Command: "npx", Args: []string{"-y", "demo"}},
					{Name: "remote", Key: a.Key("remote", ""), Transport: "http", URL: "https://example.com/mcp"},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(rendered.Content, "local") || !strings.Contains(rendered.Content, "remote") {
				t.Fatalf("content = %s", rendered.Content)
			}
			servers, err := a.Plugin.LoadServers(context.Background(), plugin.LoadContext{
				Content: rendered.Content, Document: rendered.Document, Path: a.Path(), OS: "darwin",
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(servers) != 2 {
				t.Fatalf("loaded %d servers: %+v", len(servers), servers)
			}
		})
	}
}

func TestUserPluginOverridesBuiltin(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "plugins", "cursor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"id":"cursor","label":"Cursor Custom","paths":{"darwin":"~/.cursor/custom.json"},"format":"json","section":"mcpServers"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(`export function render(ctx) { const doc = ctx.document || {}; doc.mcpServers ??= {}; doc.mcpServers[ctx.entry || "mymcp"] = {url: ctx.endpoint}; return doc; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	registry, err := New(home)
	if err != nil {
		t.Fatal(err)
	}
	cursor := registry.Get("cursor")
	if cursor == nil || cursor.Label() != "Cursor Custom" || cursor.Builtin {
		t.Fatalf("cursor = %+v", cursor)
	}
	if got := len(registry.UserPlugins()); got != 1 {
		t.Fatalf("user plugins = %d", got)
	}
	for _, plugin := range registry.BuiltinPlugins() {
		if plugin.Manifest.ID == "cursor" {
			t.Fatal("built-in cursor was not shadowed")
		}
	}
}
