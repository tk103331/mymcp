package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestServerProfilesAndCalls(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "mymcp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.EnsureDefaultProfile(ctx); err != nil {
		t.Fatal(err)
	}
	server, err := db.UpsertServer(ctx, &Server{
		Name:      "github",
		Transport: "stdio",
		Command:   "npx",
		Args:      []string{"-y", "github-mcp"},
		Env:       map[string]string{"TOKEN": "secret://github/token"},
		OAuth:     &OAuthConfig{AuthorizationURL: "https://auth", TokenURL: "https://token", ClientID: "id"},
		Enabled:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if server.ID == "" {
		t.Fatal("server has no id")
	}
	if err := db.AttachServer(ctx, "default", "github", true); err != nil {
		t.Fatal(err)
	}
	servers, err := db.ServersForProfile(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 || servers[0].Name != "github" {
		t.Fatalf("servers = %+v", servers)
	}
	if servers[0].Env["TOKEN"] != "secret://github/token" {
		t.Errorf("env ref lost: %+v", servers[0].Env)
	}
	if servers[0].OAuth == nil || servers[0].OAuth.ClientID != "id" {
		t.Errorf("oauth config lost: %+v", servers[0].OAuth)
	}

	call := &Call{Profile: "default", Server: "github", Tool: "search", Method: "tools/call", Status: "ok", LatencyMS: 12}
	if err := db.RecordCall(ctx, call); err != nil {
		t.Fatal(err)
	}
	recent, err := db.RecentCalls(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 1 || recent[0].Tool != "search" {
		t.Fatalf("recent calls = %+v", recent)
	}
	stats, err := db.CallStats(ctx, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 || stats[0].Calls != 1 || stats[0].Server != "github" {
		t.Fatalf("stats = %+v", stats)
	}
}
