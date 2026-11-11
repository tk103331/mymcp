package portable

import (
	"context"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"

	"github.com/tk103331/mymcp/internal/store"
)

func TestExportImport(t *testing.T) {
	ctx := context.Background()
	source, err := store.Open(filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if _, err := source.UpsertProfile(ctx, &store.Profile{Name: "work", Description: "Work tools"}); err != nil {
		t.Fatal(err)
	}
	for _, server := range []*store.Server{
		{Name: "demo", Transport: "stdio", Command: "npx", Args: []string{"-y", "demo"}, Env: map[string]string{"TOKEN": "secret://demo/token"}, Enabled: true},
		{Name: "remote", Transport: "http", URL: "https://example.com/mcp", Enabled: true},
	} {
		if _, err := source.UpsertServer(ctx, server); err != nil {
			t.Fatal(err)
		}
		if err := source.AttachServer(ctx, "work", server.Name, true); err != nil {
			t.Fatal(err)
		}
	}
	bundle, err := Export(ctx, source, "work")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Encode(bundle)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}

	target, err := store.Open(filepath.Join(t.TempDir(), "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	created, updated, skipped, err := Import(ctx, target, decoded, "shared", false)
	if err != nil {
		t.Fatal(err)
	}
	if created != 2 || updated != 0 || skipped != 0 {
		t.Fatalf("import = %d %d %d", created, updated, skipped)
	}
	servers, err := target.AllServersForProfile(ctx, "shared")
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 2 {
		t.Fatalf("servers = %+v", servers)
	}
	for _, server := range servers {
		if server.Name == "demo" && server.Env["TOKEN"] != "secret://demo/token" {
			t.Fatalf("secret ref lost: %+v", server)
		}
	}

	// Importing again keeps existing servers unless forced.
	created, updated, skipped, err = Import(ctx, target, decoded, "shared", false)
	if err != nil {
		t.Fatal(err)
	}
	if skipped != 2 || created != 0 || updated != 0 {
		t.Fatalf("second import = %d %d %d", created, updated, skipped)
	}
}

func TestSignVerify(t *testing.T) {
	pair, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	private := mustPrivate(t, pair.Private)
	public := mustPublic(t, pair.Public)
	bundle := &Bundle{Version: Version, Profile: store.Profile{Name: "work"}, Servers: []*store.Server{{Name: "demo", Transport: "stdio", Command: "npx"}}}
	if err := Sign(bundle, private); err != nil {
		t.Fatal(err)
	}
	if bundle.Signature == "" {
		t.Fatal("no signature")
	}
	if err := Verify(bundle, public); err != nil {
		t.Fatal(err)
	}
	bundle.Servers[0].Command = "evil"
	if err := Verify(bundle, public); err == nil {
		t.Fatal("tampered bundle verified")
	}
}

func mustPrivate(t *testing.T, raw []byte) ed25519.PrivateKey {
	t.Helper()
	path := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	key, err := ReadPrivateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func mustPublic(t *testing.T, raw []byte) ed25519.PublicKey {
	t.Helper()
	path := filepath.Join(t.TempDir(), "key.pub")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	key, err := ReadPublicKey(path)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestExportRejectsLiteralSecrets(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.UpsertProfile(ctx, &store.Profile{Name: "work"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpsertServer(ctx, &store.Server{
		Name: "demo", Transport: "stdio", Command: "npx",
		Env: map[string]string{"GITHUB_TOKEN": "plaintext"}, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.AttachServer(ctx, "work", "demo", true); err != nil {
		t.Fatal(err)
	}
	if _, err := Export(ctx, db, "work"); err == nil {
		t.Fatal("exported a literal secret")
	}
}
