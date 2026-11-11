package importer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tk103331/mymcp/internal/configsync"
	"github.com/tk103331/mymcp/internal/store"
)

func TestImportAttachesExistingServerToAnotherProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cursor := filepath.Join(home, ".cursor", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(cursor), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cursor, []byte(`{"mcpServers":{"demo":{"url":"https://example.com/mcp"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(filepath.Join(home, ".mymcp", "mymcp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sync, err := configsync.New(filepath.Join(home, ".mymcp"))
	if err != nil {
		t.Fatal(err)
	}
	stats, err := Import(context.Background(), db, sync, "work", "cursor", false)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Imported != 1 {
		t.Fatalf("first import = %+v", stats)
	}
	stats, err = Import(context.Background(), db, sync, "other", "cursor", false)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Existing != 1 || stats.Imported != 0 {
		t.Fatalf("second import = %+v", stats)
	}
	servers, err := db.ServersForProfile(context.Background(), "other")
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 || servers[0].Name != "demo" {
		t.Fatalf("other profile servers = %+v", servers)
	}
}
