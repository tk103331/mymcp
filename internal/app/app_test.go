package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/tk103331/mymcp/internal/data"
	"github.com/tk103331/mymcp/internal/gateway"
	"github.com/tk103331/mymcp/internal/model"
	"github.com/tk103331/mymcp/internal/secrets"
	"github.com/tk103331/mymcp/internal/store"
)

func newTestApp() *app {
	return &app{
		router:   ui.NewRouter("/overview"),
		settings: data.Settings{Language: "en", Theme: "light", BaseURL: "http://localhost:8421", GatewayAddr: "127.0.0.1:8422"},
		catalog:  loadCatalog(),
		gwAddr:   "127.0.0.1:8422",
	}
}

func TestCatalog(t *testing.T) {
	catalog := loadCatalog()
	if len(catalog.Servers()) == 0 {
		t.Fatal("the catalog is empty")
	}
	if len(catalog.Categories()) == 0 {
		t.Fatal("the catalog has no categories")
	}
	firecrawl := catalog.Server("firecrawl")
	if firecrawl == nil {
		t.Fatal("firecrawl is missing")
	}
	entry, err := catalogStoreServer(firecrawl, "npm", map[string]string{"FIRECRAWL_API_KEY": "secret://firecrawl"})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Command != "npx" || entry.Env["FIRECRAWL_API_KEY"] != "secret://firecrawl" {
		t.Fatalf("catalog entry = %+v", entry)
	}
}

func TestViewRenders(t *testing.T) {
	a := newTestApp()
	tt := ui.NewTester(a.view, 1000, 700)
	if !tt.HasText("Overview") {
		t.Fatalf("the overview page did not render: %q", tt.Texts())
	}
	if err := tt.Click("Catalog"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Catalog") {
		t.Fatalf("the catalog page did not render: %q", tt.Texts())
	}
	if err := tt.Click("Settings"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Gateway address") {
		t.Fatalf("the settings page did not render: %q", tt.Texts())
	}
}

func TestCreateWorkspaceDialog(t *testing.T) {
	a := newTestApp()
	a.router.Push("/workspaces")
	tt := ui.NewTester(a.view, 1000, 700)
	if err := tt.Click("Create Workspace"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Description") {
		t.Fatalf("the dialog did not open: %q", tt.Texts())
	}
}

func TestAddServiceDialog(t *testing.T) {
	a := newTestApp()
	a.router.Transition = ui.TransitionNone
	a.current = &data.Workspace{ID: "w1", Name: "Demo"}
	a.router.Push("/workspaces/w1")
	tt := ui.NewTester(a.view, 1000, 700)
	if err := tt.Click("Add Service"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Search Service") {
		t.Fatalf("the add dialog did not open: %q", tt.Texts())
	}
	if box, ok := tt.Find("Ableton Live"); ok {
		tt.ClickAt(box.X+box.W/2, box.Y+box.H/2)
	} else {
		t.Fatal("the service row did not render")
	}
	if a.addSelected == "" {
		t.Fatalf("clicking a row did not select a service")
	}
	if err := tt.Click("Next"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Installation Type") {
		t.Fatalf("the configuration step did not render: %q", tt.Texts())
	}
}

func TestInstallDialog(t *testing.T) {
	a := newTestApp()
	a.installOpen = true
	a.openInstall("firecrawl")
	tt := ui.NewTester(a.view, 1000, 700)
	if !tt.HasText("Installation Type") {
		t.Fatalf("the install dialog did not render: %q", tt.Texts())
	}
}

func TestWorkspaceDetailRenders(t *testing.T) {
	a := newTestApp()
	a.router.Transition = ui.TransitionNone
	a.current = &data.Workspace{ID: "w1", Name: "Local Tools", Description: "Tools"}
	a.instances = []*data.ServerInstance{
		{ID: "s1", Config: &model.ServerConfig{ID: "s1", Name: "firecrawl", Type: "npx"}, Status: "stopped"},
	}
	a.router.Push("/workspaces/w1")
	tt := ui.NewTester(a.view, 1000, 700)
	if !tt.HasText("Add Service") || !tt.HasText("firecrawl") {
		t.Fatalf("the workspace detail did not render: %q", tt.Texts())
	}
}

func TestAddServerConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a := newTestApp()
	a.current = &data.Workspace{ID: "w1", Name: "Local Tools"}
	if err := a.addServerConfig("w1", "firecrawl", "npm", map[string]string{"FIRECRAWL_API_KEY": "test-key"}); err != nil {
		t.Fatal(err)
	}
	configs, err := data.LoadServerConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Fatalf("got %d server configs, want 1", len(configs))
	}
	cfg := configs[0]
	if cfg.Name != "firecrawl" || cfg.Workspace != "w1" || cfg.Type != "npm" {
		t.Errorf("saved config is %+v", cfg)
	}
	if cfg.Params["FIRECRAWL_API_KEY"] != "test-key" {
		t.Errorf("the parameter was not saved: %+v", cfg.Params)
	}
	if cfg.Cmd == "" {
		t.Error("the command is empty")
	}
}

func TestClientIcons(t *testing.T) {
	loadClientIcons()
	for _, name := range []string{"cursor", "windsurf", "trae", "claude", "claudecode", "codex", "gemini", "opencode", "copilot", "cline", "kiro", "qoder", "vscode", "lmstudio", "droid", "cherrystudio", "zed", "goose", "kimi", "codebuddy", "antigravity"} {
		if clientByName(name) != nil {
			if clientByName(name).SVG() == nil {
				t.Errorf("%s has no Magpie SVG icon", name)
			}
		} else if clientSVGs[name] == nil {
			t.Errorf("%s has no Magpie SVG icon", name)
		}
	}
	for _, name := range []string{"chatwise", "deepchat", "crush"} {
		if clientByName(name).Icon() == nil {
			t.Errorf("%s has no fallback bitmap icon", name)
		}
	}
}

func TestUsagePage(t *testing.T) {
	a := newTestApp()
	a.router.Transition = ui.TransitionNone
	a.usageSince = 24 * time.Hour
	db, err := store.Open(filepath.Join(t.TempDir(), "mymcp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a.usage = db
	if err := db.RecordCall(context.Background(), &store.Call{
		Profile: "default", Server: "demo", Tool: "echo", Method: "tools/call", Status: "ok", LatencyMS: 12,
	}); err != nil {
		t.Fatal(err)
	}
	a.router.Push("/usage")
	a.loadUsage()
	tt := ui.NewTester(a.view, 1100, 720)
	if !tt.HasText("Usage") || !tt.HasText("echo") {
		t.Fatalf("usage page = %q", tt.Texts())
	}
}

func TestChineseInterface(t *testing.T) {
	a := newTestApp()
	a.router.Transition = ui.TransitionNone
	tt := ui.NewTester(a.view, 1100, 720)
	tt.SetLocale("zh-Hans")
	tt.Frame()
	for _, text := range []string{"概览", "服务器", "服务目录", "配置组", "客户端", "调试", "密钥", "插件", "用量", "设置"} {
		if !tt.HasText(text) {
			t.Fatalf("missing Chinese text %q: %q", text, tt.Texts())
		}
	}
	if err := tt.Click("插件"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("还没有安装插件") {
		t.Fatalf("plugins page is not Chinese: %q", tt.Texts())
	}
}

func TestThemeChangeAppliesImmediately(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	a := newTestApp()
	a.router.Transition = ui.TransitionNone
	a.router.Push("/settings")
	tt := ui.NewTester(a.view, 1100, 800)
	tt.SetLocale("en")
	tt.Frame()
	if err := tt.Click("Light"); err != nil {
		t.Fatalf("open theme select: %v (texts %q)", err, tt.Texts())
	}
	tt.Frame()
	if err := tt.Click("Dark"); err != nil {
		t.Fatalf("choose dark: %v (texts %q)", err, tt.Texts())
	}
	tt.Frame()
	if a.settings.Theme != "dark" {
		t.Fatalf("theme = %q, want dark", a.settings.Theme)
	}
	saved, err := data.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if saved.Theme != "dark" {
		t.Fatalf("saved theme = %q, want dark", saved.Theme)
	}
}

func TestSecretsPage(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	vault, err := secrets.Open(filepath.Join(home, ".mymcp"))
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.Set("github-token", "secret-value"); err != nil {
		t.Fatal(err)
	}
	a := newTestApp()
	a.secretsStore = vault
	a.router.Transition = ui.TransitionNone
	a.router.Push("/secrets")
	tt := ui.NewTester(a.view, 1100, 720)
	tt.SetLocale("zh-Hans")
	tt.Frame()
	if !tt.HasText("密钥") || !tt.HasText("github-token") {
		t.Fatalf("secrets page did not render: %q", tt.Texts())
	}
}

func TestNormalizeGatewayAddr(t *testing.T) {
	valid := map[string]string{
		"127.0.0.1:8422": "127.0.0.1:8422",
		"localhost:8422": "localhost:8422",
		"[::1]:8422":     "[::1]:8422",
	}
	for input, want := range valid {
		got, err := normalizeGatewayAddr(input)
		if err != nil || got != want {
			t.Errorf("normalizeGatewayAddr(%q) = %q, %v", input, got, err)
		}
	}
	for _, input := range []string{"", "127.0.0.1", "0.0.0.0:8422", "127.0.0.1:0", "127.0.0.1:99999", "example.com:8422"} {
		if _, err := normalizeGatewayAddr(input); err == nil {
			t.Errorf("normalizeGatewayAddr(%q) was accepted", input)
		}
	}
}

func TestDebugPage(t *testing.T) {
	a := newTestApp()
	a.gwRunning = true
	a.gwProfiles = []*store.Profile{{Name: "default"}}
	a.debugProfile = "default"
	a.debugTools = []gateway.ToolInfo{{
		Name: "demo__echo", Server: "demo", Tool: "echo",
		Description: "Echo text", InputSchema: []byte(`{"type":"object","properties":{"text":{"type":"string"}}}`),
	}}
	a.debugSelected = "demo__echo"
	a.debugArgs = `{"text":"hello"}`
	a.router.Transition = ui.TransitionNone
	a.router.Push("/debug")
	a.lastPath = "/debug"
	tt := ui.NewTester(a.view, 1100, 800)
	tt.SetLocale("zh-Hans")
	tt.Frame()
	for _, text := range []string{"MCP 调试", "demo__echo", "参数结构", "调用工具"} {
		if !tt.HasText(text) {
			t.Fatalf("debug page is missing %q: %q", text, tt.Texts())
		}
	}
}
