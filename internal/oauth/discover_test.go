package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscoverAndRegister(t *testing.T) {
	var server *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"resource":"` + server.URL + `/mcp","authorization_servers":["` + server.URL + `"],"scopes_supported":["mcp.read"]}`))
	})
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"issuer":"` + server.URL + `","authorization_endpoint":"` + server.URL + `/authorize","token_endpoint":"` + server.URL + `/token","registration_endpoint":"` + server.URL + `/register"}`))
	})
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("register method = %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"client_id":"client-1","client_secret":"secret-1"}`))
	})
	server = httptest.NewServer(mux)
	defer server.Close()

	ctx := context.Background()
	resource, err := Discover(ctx, server.URL+"/mcp", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if len(resource.AuthorizationServers) != 1 || resource.AuthorizationServers[0] != server.URL {
		t.Fatalf("resource = %+v", resource)
	}
	auth, err := AuthServer(ctx, resource.AuthorizationServers[0], server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if auth.AuthorizationEndpoint != server.URL+"/authorize" || auth.RegistrationEndpoint != server.URL+"/register" {
		t.Fatalf("auth = %+v", auth)
	}
	id, secret, err := Register(ctx, auth, "http://127.0.0.1:33418/callback", "MyMCP", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if id != "client-1" || secret != "secret-1" {
		t.Fatalf("client = %q %q", id, secret)
	}
}
