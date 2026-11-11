package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/tk103331/mymcp/internal/store"
)

func TestAuthMiddleware(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "mymcp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	token, _, err := db.CreateKey(ctx, "test", "default", 2, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	local := &http.Request{URL: mustURL("/mcp/default"), RemoteAddr: "127.0.0.1:1234", Header: http.Header{}}
	rec := httptest.NewRecorder()
	NewAuth(db, false).Wrap(handler).ServeHTTP(rec, local)
	if rec.Code != http.StatusOK {
		t.Fatalf("local without key = %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	NewAuth(db, true).Wrap(handler).ServeHTTP(rec, local)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("local with require-key = %d", rec.Code)
	}

	remote := &http.Request{URL: mustURL("/mcp/default"), RemoteAddr: "10.0.0.2:1234", Header: http.Header{}}
	rec = httptest.NewRecorder()
	NewAuth(db, false).Wrap(handler).ServeHTTP(rec, remote)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("remote without key = %d", rec.Code)
	}

	authed := &http.Request{URL: mustURL("/mcp/default"), RemoteAddr: "10.0.0.2:1234", Header: http.Header{"Authorization": {"Bearer " + token}}}
	rec = httptest.NewRecorder()
	NewAuth(db, false).Wrap(handler).ServeHTTP(rec, authed)
	if rec.Code != http.StatusOK {
		t.Fatalf("remote with key = %d", rec.Code)
	}

	// The key allows two calls a minute.
	auth := NewAuth(db, false)
	for i := 0; i < 2; i++ {
		rec = httptest.NewRecorder()
		auth.Wrap(handler).ServeHTTP(rec, authed)
		if rec.Code != http.StatusOK {
			t.Fatalf("call %d = %d", i, rec.Code)
		}
	}
	rec = httptest.NewRecorder()
	auth.Wrap(handler).ServeHTTP(rec, authed)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("rate limited = %d", rec.Code)
	}

	otherProfile := &http.Request{URL: mustURL("/mcp/work"), RemoteAddr: "127.0.0.1:1", Header: http.Header{"Authorization": {"Bearer " + token}}}
	rec = httptest.NewRecorder()
	NewAuth(db, false).Wrap(handler).ServeHTTP(rec, otherProfile)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("profile restriction = %d", rec.Code)
	}

	remoteDashboard := &http.Request{URL: mustURL("/dashboard"), RemoteAddr: "10.0.0.2:1234", Header: http.Header{"Authorization": {"Bearer " + token}}}
	rec = httptest.NewRecorder()
	NewAuth(db, false).Wrap(handler).ServeHTTP(rec, remoteDashboard)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("remote dashboard = %d", rec.Code)
	}
}

func mustURL(path string) *url.URL {
	u, _ := url.Parse(path)
	return u
}
