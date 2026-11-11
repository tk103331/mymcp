package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tk103331/mymcp/internal/secrets"
	"github.com/tk103331/mymcp/internal/store"
)

func TestAccessTokenRefreshes(t *testing.T) {
	secretStore, err := secrets.Open(filepath.Join(t.TempDir(), "secrets"))
	if err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("client/secret", "s3cret"); err != nil {
		t.Fatal(err)
	}
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("client_secret") != "s3cret" {
			t.Errorf("refresh form = %v", r.Form)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"new","refresh_token":"refresh-2","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenServer.Close()

	if err := Save(secretStore, "demo", &Token{
		AccessToken: "old", RefreshToken: "refresh-1", Expiry: time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	cfg := &store.OAuthConfig{
		TokenURL: tokenServer.URL, ClientID: "client",
		ClientSecret: "secret://client/secret",
	}
	token, err := AccessToken(context.Background(), cfg, "demo", secretStore, tokenServer.Client())
	if err != nil {
		t.Fatal(err)
	}
	if token != "new" {
		t.Fatalf("token = %q", token)
	}
	loaded, err := Load(secretStore, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.RefreshToken != "refresh-2" || loaded.AccessToken != "new" {
		t.Fatalf("stored token = %+v", loaded)
	}
}

func TestLoginFlow(t *testing.T) {
	secretStore, err := secrets.Open(filepath.Join(t.TempDir(), "secrets"))
	if err != nil {
		t.Fatal(err)
	}
	var gotCode, gotVerifier string
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		gotCode = r.Form.Get("code")
		gotVerifier = r.Form.Get("code_verifier")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"access","refresh_token":"refresh","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenServer.Close()

	cfg := &store.OAuthConfig{
		AuthorizationURL: "https://auth.example/authorize",
		TokenURL:         tokenServer.URL,
		ClientID:         "client",
		Scopes:           []string{"mcp.read", "mcp.write"},
		UsePKCE:          true,
	}
	open := func(raw string) error {
		u, err := url.Parse(raw)
		if err != nil {
			return err
		}
		q := u.Query()
		if q.Get("code_challenge") == "" || q.Get("state") == "" {
			t.Errorf("authorization query = %v", q)
		}
		redirect := q.Get("redirect_uri")
		resp, err := http.Get(redirect + "?code=abc&state=" + url.QueryEscape(q.Get("state")))
		if err != nil {
			return err
		}
		resp.Body.Close()
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	token, err := Login(ctx, cfg, "demo", secretStore, open, &strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "access" {
		t.Fatalf("token = %+v", token)
	}
	if gotCode != "abc" || gotVerifier == "" {
		t.Fatalf("code = %q, verifier = %q", gotCode, gotVerifier)
	}
	loaded, err := Load(secretStore, "demo")
	if err != nil || loaded.AccessToken != "access" {
		t.Fatalf("stored = %+v, %v", loaded, err)
	}
}
