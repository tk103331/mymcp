package secrets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVaultRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set("github/token", "ghp_secret"); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("openai/key", "sk-secret"); err != nil {
		t.Fatal(err)
	}
	value, err := store.Get("github/token")
	if err != nil || value != "ghp_secret" {
		t.Fatalf("get = %q, %v", value, err)
	}
	names, err := store.List()
	if err != nil || len(names) != 2 {
		t.Fatalf("list = %v, %v", names, err)
	}
	resolved, err := store.Resolve("secret://github/token")
	if err != nil || resolved != "ghp_secret" {
		t.Fatalf("resolve = %q, %v", resolved, err)
	}
	literal, err := store.Resolve("plain")
	if err != nil || literal != "plain" {
		t.Fatalf("literal = %q, %v", literal, err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "secrets.enc"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "ghp_secret") || strings.Contains(string(raw), "sk-secret") {
		t.Fatal("the vault file contains plaintext")
	}

	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if value, err := reopened.Get("openai/key"); err != nil || value != "sk-secret" {
		t.Fatalf("reopened get = %q, %v", value, err)
	}
	if err := reopened.Delete("openai/key"); err != nil {
		t.Fatal(err)
	}
	if reopened.Has("openai/key") {
		t.Fatal("deleted secret is still there")
	}
}
