package configsync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

func TestApplyPlanDetachJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".cursor", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"mcpServers":{"keep":{"url":"http://keep"}},"theme":"dark"}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := New(filepath.Join(home, ".mymcp"))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := manager.Plan("cursor", "http://127.0.0.1:8422/mcp/default", "mymcp")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Exists || plan.Conflict || plan.File != path {
		t.Fatalf("plan = %+v", plan)
	}
	if _, err := manager.Apply("cursor", "http://127.0.0.1:8422/mcp/default", "mymcp", false); err != nil {
		t.Fatal(err)
	}
	doc := readJSON(t, path)
	servers := doc["mcpServers"].(map[string]any)
	if _, ok := servers["keep"]; !ok {
		t.Fatalf("keep was removed: %v", doc)
	}
	entry := servers["mymcp"].(map[string]any)
	if entry["url"] != "http://127.0.0.1:8422/mcp/default" {
		t.Fatalf("entry = %v", entry)
	}
	if doc["theme"] != "dark" {
		t.Fatalf("theme was removed: %v", doc)
	}

	// A user edit makes the next apply a conflict.
	entry["url"] = "http://user-edited"
	writeJSON(t, path, doc)
	conflict, err := manager.Plan("cursor", "http://127.0.0.1:8422/mcp/default", "mymcp")
	if err != nil {
		t.Fatal(err)
	}
	if !conflict.Conflict {
		t.Fatalf("plan did not see the conflict: %+v", conflict)
	}
	if _, err := manager.Apply("cursor", "http://127.0.0.1:8422/mcp/default", "mymcp", false); err == nil {
		t.Fatal("apply overwrote a user-edited entry")
	}
	if _, err := manager.Apply("cursor", "http://127.0.0.1:8422/mcp/default", "mymcp", true); err != nil {
		t.Fatal(err)
	}

	if err := manager.Detach("cursor", false); err != nil {
		t.Fatal(err)
	}
	doc = readJSON(t, path)
	servers = doc["mcpServers"].(map[string]any)
	if _, ok := servers["mymcp"]; ok {
		t.Fatalf("detach left the entry: %v", doc)
	}
	if _, ok := servers["keep"]; !ok {
		t.Fatalf("detach removed keep: %v", doc)
	}
}

func TestApplyCodexTOML(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("model = \"o3\"\n\n[mcp_servers.keep]\ncommand = \"keep\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := New(filepath.Join(home, ".mymcp"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Apply("codex", "http://127.0.0.1:8422/mcp/default", "mymcp", false); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := toml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	servers := doc["mcp_servers"].(map[string]any)
	if _, ok := servers["keep"]; !ok {
		t.Fatalf("keep was removed: %s", raw)
	}
	entry := servers["mymcp"].(map[string]any)
	if entry["command"] != "mcp-pipe" {
		t.Fatalf("entry = %v", entry)
	}
	if doc["model"] != "o3" {
		t.Fatalf("model was removed: %s", raw)
	}
}

func TestVSCODEAndComments(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := ClientByID("vscode").Path()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{\n  // my servers\n  \"servers\": {\"keep\": {\"url\": \"http://keep\"}}\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := New(filepath.Join(home, ".mymcp"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Apply("vscode", "http://127.0.0.1:8422/mcp/default", "mymcp", false); err != nil {
		t.Fatal(err)
	}
	doc := readJSON(t, path)
	servers := doc["servers"].(map[string]any)
	entry := servers["mymcp"].(map[string]any)
	if entry["type"] != "sse" {
		t.Fatalf("entry = %v", entry)
	}
	if !strings.Contains(string(mustRead(t, path)), "\"keep\"") {
		t.Fatal("keep was removed")
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(mustRead(t, path), &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func writeJSON(t *testing.T, path string, doc map[string]any) {
	t.Helper()
	raw, _ := json.MarshalIndent(doc, "", "  ")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestApplyDirectJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".cursor", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"keep":{"url":"http://keep"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := New(filepath.Join(home, ".mymcp"))
	if err != nil {
		t.Fatal(err)
	}
	servers := []DirectServer{
		{Name: "demo", Transport: "stdio", Command: "npx", Args: []string{"-y", "demo"}},
		{Name: "remote", Transport: "http", URL: "https://example.com/mcp"},
	}
	if _, err := manager.ApplyDirect("cursor", servers, false); err != nil {
		t.Fatal(err)
	}
	doc := readJSON(t, path)
	section := doc["mcpServers"].(map[string]any)
	demo := section["demo"].(map[string]any)
	if demo["command"] != "npx" {
		t.Fatalf("demo = %v", demo)
	}
	remote := section["remote"].(map[string]any)
	if remote["url"] != "https://example.com/mcp" {
		t.Fatalf("remote = %v", remote)
	}
	if _, ok := section["keep"]; !ok {
		t.Fatalf("keep was removed: %v", doc)
	}
	if err := manager.Detach("cursor", false); err != nil {
		t.Fatal(err)
	}
	doc = readJSON(t, path)
	section = doc["mcpServers"].(map[string]any)
	if _, ok := section["demo"]; ok {
		t.Fatalf("demo was not removed: %v", doc)
	}
	if _, ok := section["keep"]; !ok {
		t.Fatalf("keep was removed by detach: %v", doc)
	}
}

func TestDirectRefusesSecrets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	manager, err := New(filepath.Join(home, ".mymcp"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.PlanDirect("cursor", []DirectServer{{
		Name: "demo", Transport: "stdio", Command: "npx",
		Env: map[string]string{"TOKEN": "secret://demo/token"},
	}}, false)
	if err == nil {
		t.Fatal("direct mode accepted a secret reference")
	}
}

func TestDirectConflict(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".cursor", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"demo":{"url":"http://mine"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := New(filepath.Join(home, ".mymcp"))
	if err != nil {
		t.Fatal(err)
	}
	servers := []DirectServer{{Name: "demo", Transport: "http", URL: "https://example.com/mcp"}}
	if _, err := manager.ApplyDirect("cursor", servers, false); err == nil {
		t.Fatal("direct mode overwrote an unmanaged entry")
	}
	if _, err := manager.ApplyDirect("cursor", servers, true); err != nil {
		t.Fatal(err)
	}
}

func TestDirectCodexRemoteUsesBridge(t *testing.T) {
	entry := directEntry("codex", DirectServer{Name: "remote", Transport: "http", URL: "https://example.com/mcp"})
	if entry["command"] != "mcp-pipe" {
		t.Fatalf("codex remote entry = %v", entry)
	}
}

func TestGooseYAML(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".config", "goose", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("extensions:\n  keep:\n    enabled: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := New(filepath.Join(home, ".mymcp"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Apply("goose", "http://127.0.0.1:8422/mcp/default", "mymcp", false); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	extensions := doc["extensions"].(map[string]any)
	if _, ok := extensions["keep"]; !ok {
		t.Fatalf("keep was removed: %s", raw)
	}
	entry := extensions["mymcp"].(map[string]any)
	if entry["type"] != "sse" || entry["uri"] != "http://127.0.0.1:8422/mcp/default/sse" {
		t.Fatalf("entry = %v", entry)
	}
}

func TestNewClientShapes(t *testing.T) {
	zed := directEntry("zed", DirectServer{Name: "remote", Transport: "sse", URL: "https://example.com/sse"})
	if zed["command"] != "mcp-pipe" {
		t.Fatalf("zed remote = %v", zed)
	}
	kimi := directEntry("kimi", DirectServer{Name: "remote", Transport: "sse", URL: "https://example.com/sse"})
	if kimi["transport"] != "sse" {
		t.Fatalf("kimi = %v", kimi)
	}
	antigravity := directEntry("antigravity", DirectServer{Name: "remote", Transport: "sse", URL: "https://example.com/sse"})
	if antigravity["serverUrl"] != "https://example.com/sse" {
		t.Fatalf("antigravity = %v", antigravity)
	}
	goose := directEntry("goose", DirectServer{Name: "demo", Transport: "stdio", Command: "npx", Args: []string{"-y", "demo"}})
	if goose["type"] != "stdio" || goose["cmd"] != "npx" {
		t.Fatalf("goose = %v", goose)
	}
}

func TestSSEClientsUseSSEEndpoint(t *testing.T) {
	endpoint := "http://127.0.0.1:8422/mcp/default"
	for _, id := range []string{"claudecode", "copilot", "cline", "qoder", "crush", "vscode", "droid", "goose", "kimi", "codebuddy", "antigravity"} {
		client := ClientByID(id)
		if client == nil {
			t.Fatalf("missing client %q", id)
		}
		raw, err := json.Marshal(client.Entry(endpoint))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), endpoint+"/sse") {
			t.Errorf("%s entry = %s, want the SSE endpoint", id, raw)
		}
	}
	claude := ClientByID("claude")
	entry := claude.Entry(endpoint)
	args, _ := entry["args"].([]string)
	if len(args) == 0 || args[len(args)-1] != endpoint+"/sse" {
		t.Fatalf("claude bridge = %v", entry)
	}
}

func TestManagedClientDrift(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	manager, err := New(filepath.Join(home, ".mymcp"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Apply("cursor", "http://127.0.0.1:8422/mcp/default", "mymcp", false); err != nil {
		t.Fatal(err)
	}
	drift, err := manager.Drift("cursor")
	if err != nil || drift {
		t.Fatalf("fresh entry: drift=%v err=%v", drift, err)
	}
	path := filepath.Join(home, ".cursor", "mcp.json")
	doc := readJSON(t, path)
	doc["mcpServers"].(map[string]any)["mymcp"].(map[string]any)["url"] = "http://user-edited"
	writeJSON(t, path, doc)
	drift, err = manager.Drift("cursor")
	if err != nil || !drift {
		t.Fatalf("edited entry: drift=%v err=%v", drift, err)
	}
	for _, status := range manager.Statuses() {
		if status.Client == "cursor" && !status.Drifted {
			t.Fatalf("status did not report drift: %+v", status)
		}
	}
}

func TestRestoreBackup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".cursor", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	original := `{"mcpServers":{"keep":{"url":"http://keep"}}}`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := New(filepath.Join(home, ".mymcp"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Apply("cursor", "http://127.0.0.1:8422/mcp/default", "mymcp", false); err != nil {
		t.Fatal(err)
	}
	if err := manager.Restore("cursor"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != original {
		t.Fatalf("restored %q, want %q", raw, original)
	}
	for _, status := range manager.Statuses() {
		if status.Client == "cursor" && status.Managed {
			t.Fatal("restore left the client managed")
		}
	}
}

func TestRouterApplyRemovesDirectEntries(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	manager, err := New(filepath.Join(home, ".mymcp"))
	if err != nil {
		t.Fatal(err)
	}
	direct := []DirectServer{{Name: "demo", Transport: "stdio", Command: "npx", Args: []string{"-y", "demo"}}}
	if _, err := manager.ApplyDirect("cursor", direct, false); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Apply("cursor", "http://127.0.0.1:8422/mcp/default", "mymcp", false); err != nil {
		t.Fatal(err)
	}
	doc := readJSON(t, filepath.Join(home, ".cursor", "mcp.json"))
	servers := doc["mcpServers"].(map[string]any)
	if _, ok := servers["demo"]; ok {
		t.Fatalf("direct entry survived router apply: %v", servers)
	}
	if _, ok := servers["mymcp"]; !ok {
		t.Fatalf("router entry missing: %v", servers)
	}
}

func TestRouterApplySeesDirectEntryConflict(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	manager, err := New(filepath.Join(home, ".mymcp"))
	if err != nil {
		t.Fatal(err)
	}
	direct := []DirectServer{{Name: "demo", Transport: "stdio", Command: "npx"}}
	if _, err := manager.ApplyDirect("cursor", direct, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".cursor", "mcp.json")
	doc := readJSON(t, path)
	doc["mcpServers"].(map[string]any)["demo"].(map[string]any)["command"] = "user-edited"
	writeJSON(t, path, doc)
	plan, err := manager.Plan("cursor", "http://127.0.0.1:8422/mcp/default", "mymcp")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Conflict {
		t.Fatalf("plan missed the direct entry conflict: %+v", plan)
	}
	if _, err := manager.Apply("cursor", "http://127.0.0.1:8422/mcp/default", "mymcp", false); err == nil {
		t.Fatal("router apply overwrote a user-edited direct entry")
	}
}
