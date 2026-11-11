package data

import (
	"testing"

	"github.com/tk103331/mymcp/internal/model"
)

func TestSettingsDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	settings, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.Theme != "system" {
		t.Fatalf("theme = %q, want system", settings.Theme)
	}
	if settings.Language != "zh" {
		t.Fatalf("language = %q, want zh", settings.Language)
	}
	if settings.BaseURL != "http://localhost:8421" {
		t.Fatalf("base URL = %q", settings.BaseURL)
	}
	if settings.GatewayAddr != "127.0.0.1:8422" {
		t.Fatalf("gateway address = %q", settings.GatewayAddr)
	}
}

func TestDeleteWorkspaceRemovesItsServers(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	workspace := &Workspace{Name: "demo", Description: "demo"}
	if err := SaveWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	config := &model.ServerConfig{Workspace: workspace.ID, Name: "demo-server", Cmd: "demo"}
	if err := SaveServerConfig(config); err != nil {
		t.Fatal(err)
	}
	if err := DeleteWorkspace(workspace.ID); err != nil {
		t.Fatal(err)
	}
	configs, err := LoadServerConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 0 {
		t.Fatalf("orphan server configs: %+v", configs)
	}
}
