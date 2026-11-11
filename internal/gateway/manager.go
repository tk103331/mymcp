package gateway

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/tk103331/mymcp/internal/secrets"
	"github.com/tk103331/mymcp/internal/store"
)

// Manager serves every profile of one MyMCP data directory.
type Manager struct {
	store    *store.DB
	secrets  *secrets.Store
	baseURL  string
	gateways map[string]*Gateway
	mux      *http.ServeMux
}

// ManagerOptions configures every profile's gateway.
type ManagerOptions struct {
	BaseURL             string
	ToolSearch          bool
	ToolSearchThreshold int
}

// NewManager builds the gateways of every profile.
func NewManager(ctx context.Context, db *store.DB, secretStore *secrets.Store, opts ManagerOptions) (*Manager, error) {
	if _, err := db.EnsureDefaultProfile(ctx); err != nil {
		return nil, err
	}
	profiles, err := db.ListProfiles(ctx)
	if err != nil {
		return nil, err
	}
	m := &Manager{
		store:    db,
		secrets:  secretStore,
		baseURL:  opts.BaseURL,
		gateways: map[string]*Gateway{},
		mux:      http.NewServeMux(),
	}
	for _, profile := range profiles {
		g, err := New(Options{
			Profile: profile.Name, Store: db, Secrets: secretStore, BaseURL: opts.BaseURL,
			ToolSearch: opts.ToolSearch, ToolSearchThreshold: opts.ToolSearchThreshold,
		})
		if err != nil {
			m.Close()
			return nil, err
		}
		m.gateways[profile.Name] = g
		m.mux.Handle("/mcp/"+profile.Name, g.Handler())
		m.mux.Handle("/mcp/"+profile.Name+"/", g.Handler())
	}
	m.mux.HandleFunc("/healthz", m.health)
	m.mux.HandleFunc("/metrics", m.metrics)
	m.mux.HandleFunc("/dashboard", m.dashboard)
	m.mux.HandleFunc("/api/usage/summary", m.usageSummary)
	m.mux.HandleFunc("/api/usage/series", m.usageSeries)
	m.mux.HandleFunc("/api/usage/calls", m.usageCalls)
	m.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/dashboard", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})
	return m, nil
}

// Handler serves every profile and the health endpoint.
func (m *Manager) Handler() http.Handler { return m.mux }

// BaseURL is the address clients use.
func (m *Manager) BaseURL() string { return m.baseURL }

// Statuses reports each profile.
func (m *Manager) Statuses() map[string]Status {
	out := make(map[string]Status, len(m.gateways))
	for name, g := range m.gateways {
		out[name] = g.Status()
	}
	return out
}

// Close ends every upstream.
func (m *Manager) Close() {
	for name, g := range m.gateways {
		log.Printf("closing profile %s", name)
		g.Close()
	}
}

func (m *Manager) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Version  string            `json:"version"`
		Profiles map[string]Status `json:"profiles"`
	}{Version: Version, Profiles: m.Statuses()})
}
