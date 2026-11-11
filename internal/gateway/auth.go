package gateway

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tk103331/mymcp/internal/store"
)

// Auth checks gateway keys and per-key rate limits. Loopback clients are
// allowed without a key unless require is set; other clients always need one.
type Auth struct {
	store   *store.DB
	require bool
	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	minute int64
	count  int
}

// NewAuth builds the middleware state.
func NewAuth(db *store.DB, require bool) *Auth {
	return &Auth{store: db, require: require, buckets: map[string]*bucket{}}
}

// Wrap protects next.
func (a *Auth) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		remote := !loopbackRequest(r)
		// The dashboard and usage API are local admin surfaces, not part of
		// the MCP endpoint. Keep them off the network even with a key.
		if remote && !strings.HasPrefix(r.URL.Path, "/mcp/") && r.URL.Path != "/healthz" {
			http.Error(w, "not available remotely", http.StatusNotFound)
			return
		}
		if token == "" {
			if a.require || remote {
				http.Error(w, "missing gateway key", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		key, err := a.store.ValidateKey(r.Context(), token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if key.Profile != "" {
			profile := profileOfPath(r.URL.Path)
			if profile != "" && profile != key.Profile {
				http.Error(w, "key is not allowed for this profile", http.StatusForbidden)
				return
			}
		}
		if key.RatePerMin > 0 && !a.allow(key.ID, key.RatePerMin) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *Auth) allow(id string, rate int) bool {
	now := time.Now().Truncate(time.Minute).Unix()
	a.mu.Lock()
	defer a.mu.Unlock()
	b := a.buckets[id]
	if b == nil || b.minute != now {
		b = &bucket{minute: now}
		a.buckets[id] = b
	}
	if b.count >= rate {
		return false
	}
	b.count++
	return true
}

func bearer(r *http.Request) string {
	if value := r.Header.Get("X-Mymcp-Key"); value != "" {
		return strings.TrimSpace(value)
	}
	header := r.Header.Get("Authorization")
	if len(header) > 7 && strings.EqualFold(header[:7], "Bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return ""
}

func profileOfPath(path string) string {
	rest, ok := strings.CutPrefix(path, "/mcp/")
	if !ok {
		return ""
	}
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		return rest[:i]
	}
	return rest
}

func loopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
